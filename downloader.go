package crunchyroll

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"github.com/grafov/m3u8"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NewDownloader creates a downloader with default settings which should
// fit the most needs
func NewDownloader(context context.Context, writer io.Writer, goroutines int, onSegmentDownload func(segment *m3u8.MediaSegment, current, total int, file *os.File) error) Downloader {
	tmp, _ := os.MkdirTemp("", "crunchy_")

	return Downloader{
		Writer:            writer,
		TempDir:           tmp,
		DeleteTempAfter:   true,
		Context:           context,
		Goroutines:        goroutines,
		OnSegmentDownload: onSegmentDownload,
	}
}

// Downloader is used to download Format's
type Downloader struct {
	// The output is all written to Writer
	Writer io.Writer

	// TempDir is the directory where the temporary segment files should be stored.
	// The files will be placed directly into the root of the directory.
	// If empty a random temporary directory on the system's default tempdir
	// will be created.
	// If the directory does not exist, it will be created
	TempDir string
	// If DeleteTempAfter is true, the temp directory gets deleted afterwards.
	// Note that in case of a hard signal exit (os.Interrupt, ...) the directory
	// will NOT be deleted. In such situations try to catch the signal and
	// cancel Context
	DeleteTempAfter bool

	// Context to control the download process with.
	// There is a tiny delay when canceling the context and the actual stop of the
	// process. So it is not recommend stopping the program immediately after calling
	// the cancel function. It's better when canceling it and then exit the program
	// when Format.Download throws an error. See the signal handling section in
	// cmd/crunchyroll-go/cmd/download.go for an example
	Context context.Context

	// Goroutines is the number of goroutines to download segments with
	Goroutines int

	// A method to call when a segment was downloaded.
	// Note that the segments are downloaded asynchronously (depending on the count of
	// Goroutines) and the function gets called asynchronously too, so for example it is
	// first called on segment 1, then segment 254, then segment 3 and so on
	OnSegmentDownload func(segment *m3u8.MediaSegment, current, total int, file *os.File) error
	// If LockOnSegmentDownload is true, only one OnSegmentDownload function can be called at
	// once. Normally (because of the use of goroutines while downloading) multiple could get
	// called simultaneously
	LockOnSegmentDownload bool

	// If FFmpegOpts is not nil, ffmpeg will be used to merge and convert files.
	// The given opts will be used as ffmpeg parameters while merging.
	//
	// If Writer is *os.File and -f (which sets the output format) is not specified, the output
	// format will be retrieved by its file ending. If this is not the case and -f is not given,
	// the output format will be mpegts / mpeg transport stream.
	// Execute 'ffmpeg -muxers' to see all available output formats.
	FFmpegOpts []string
}

// download's the given format
func (d Downloader) download(format *Format) error {
	if err := format.InitVideo(); err != nil {
		return err
	}

	if _, err := os.Stat(d.TempDir); os.IsNotExist(err) {
		if err := os.Mkdir(d.TempDir, 0700); err != nil {
			return err
		}
	}
	if d.DeleteTempAfter {
		defer os.RemoveAll(d.TempDir)
	}

	files, err := d.downloadSegments(format)
	if err != nil {
		return err
	}
	if d.FFmpegOpts == nil {
		return d.mergeSegments(files)
	} else {
		return d.mergeSegmentsFFmpeg(files)
	}
}

// mergeSegments reads every file in tempDir and writes their content to Downloader.Writer.
// The given output file gets created or overwritten if already existing
func (d Downloader) mergeSegments(files []string) error {
	for _, file := range files {
		select {
		case <-d.Context.Done():
			return d.Context.Err()
		default:
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err = io.Copy(d.Writer, f); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// mergeSegmentsFFmpeg reads every file in tempDir and merges their content to the outputFile
// with ffmpeg (https://ffmpeg.org/).
// The given output file gets created or overwritten if already existing
func (d Downloader) mergeSegmentsFFmpeg(files []string) error {
	list, err := os.Create(filepath.Join(d.TempDir, "list.txt"))
	if err != nil {
		return err
	}

	for _, file := range files {
		if _, err = fmt.Fprintf(list, "file '%s'\n", file); err != nil {
			list.Close()
			return err
		}
	}
	list.Close()

	// predefined options ... custom options ... predefined output filename
	command := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", list.Name(),
		"-c", "copy",
	}
	if d.FFmpegOpts != nil {
		command = append(command, d.FFmpegOpts...)
	}

	var tmpfile string
	if _, ok := d.Writer.(*io.PipeWriter); ok {
		if file, ok := d.Writer.(*os.File); ok {
			tmpfile = filepath.Base(file.Name())
		}
	}
	if filepath.Ext(tmpfile) == "" {
		// checks if the -f flag is set (overwrites the output format)
		var hasF bool
		for _, opts := range d.FFmpegOpts {
			if strings.TrimSpace(opts) == "-f" {
				hasF = true
				break
			}
		}
		if !hasF {
			command = append(command, "-f", "matroska")
			f, err := os.CreateTemp(d.TempDir, "")
			if err != nil {
				return err
			}
			f.Close()
			tmpfile = f.Name()
		}
	}
	command = append(command, tmpfile)

	var errBuf bytes.Buffer
	cmd := exec.CommandContext(d.Context, "ffmpeg",
		command...)
	cmd.Stderr = &errBuf

	if err = cmd.Run(); err != nil {
		if errBuf.Len() > 0 {
			return fmt.Errorf(errBuf.String())
		} else {
			return err
		}
	}
	file, err := os.Open(tmpfile)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(d.Writer, file)
	return err
}

// downloadSegments downloads every mpeg transport stream segment to a given
// directory (more information below).
// After every segment download onSegmentDownload will be called with:
//		the downloaded segment, the current position, the total size of segments to download,
//		the file where the segment content was written to an error (if occurred).
// The filename is always <number of downloaded segment>.ts
//
// Short explanation:
// 		The actual crunchyroll video is split up in multiple segments (or video files) which
//		have to be downloaded and merged after to generate a single video file.
//		And this function just downloads each of this segment into the given directory.
// 		See https://en.wikipedia.org/wiki/MPEG_transport_stream for more information
func (d Downloader) downloadSegments(format *Format) ([]string, error) {
	if err := format.InitVideo(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var lock sync.Mutex
	chunkSize := int(math.Ceil(float64(format.Video.Chunklist.Count()) / float64(d.Goroutines)))

	// when a onSegmentDownload call returns an error, this context will be set cancelled and stop all goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// receives the decrypt block and iv from the first segment.
	// in my tests, only the first segment has specified this data, so the decryption data from this first segments will be used in every other segment too
	block, iv, err := getCrypt(format, format.Video.Chunklist.Segments[0])
	if err != nil {
		return nil, err
	}

	var total int32
	for i := 0; i < int(format.Video.Chunklist.Count()); i += chunkSize {
		wg.Add(1)
		end := i + chunkSize
		if end > int(format.Video.Chunklist.Count()) {
			end = int(format.Video.Chunklist.Count())
		}
		i := i

		go func() {
			defer wg.Done()

			for j, segment := range format.Video.Chunklist.Segments[i:end] {
				select {
				case <-d.Context.Done():
				case <-ctx.Done():
					return
				default:
					var file *os.File
					for k := 0; k < 3; k++ {
						filename := filepath.Join(d.TempDir, fmt.Sprintf("%d.ts", i+j))
						file, err = d.downloadSegment(format, segment, filename, block, iv)
						if err == nil {
							break
						}
						if k == 2 {
							cancel()
							return
						}
						select {
						case <-d.Context.Done():
						case <-ctx.Done():
							return
						case <-time.After(5 * time.Duration(k) * time.Second):
							// sleep if an error occurs. very useful because sometimes the connection times out
						}
					}
					if d.OnSegmentDownload != nil {
						if d.LockOnSegmentDownload {
							lock.Lock()
						}

						if err = d.OnSegmentDownload(segment, int(atomic.AddInt32(&total, 1)), int(format.Video.Chunklist.Count()), file); err != nil {
							if d.LockOnSegmentDownload {
								lock.Unlock()
							}
							file.Close()
							return
						}
						if d.LockOnSegmentDownload {
							lock.Unlock()
						}
					}
					file.Close()
				}
			}
		}()
	}
	wg.Wait()

	select {
	case <-d.Context.Done():
		return nil, d.Context.Err()
	case <-ctx.Done():
		return nil, err
	default:
		var files []string
		for i := 0; i < int(total); i++ {
			files = append(files, filepath.Join(d.TempDir, fmt.Sprintf("%d.ts", i)))
		}

		return files, nil
	}
}

// getCrypt extracts the key and iv of a m3u8 segment and converts it into a cipher.Block and an iv byte sequence
func getCrypt(format *Format, segment *m3u8.MediaSegment) (block cipher.Block, iv []byte, err error) {
	var resp *http.Response

	resp, err = format.crunchy.Client.Get(segment.Key.URI)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	key, err := io.ReadAll(resp.Body)

	block, err = aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	iv = []byte(segment.Key.IV)
	if len(iv) == 0 {
		iv = key
	}

	return block, iv, nil
}

// downloadSegment downloads a segment, decrypts it and names it after the given index
func (d Downloader) downloadSegment(format *Format, segment *m3u8.MediaSegment, filename string, block cipher.Block, iv []byte) (*os.File, error) {
	// every segment is aes-128 encrypted and has to be decrypted when downloaded
	content, err := d.decryptSegment(format.crunchy.Client, segment, block, iv)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err = file.Write(content); err != nil {
		return nil, err
	}

	return file, nil
}

// https://github.com/oopsguy/m3u8/blob/4150e93ec8f4f8718875a02973f5d792648ecb97/tool/crypt.go#L25
func (d Downloader) decryptSegment(client *http.Client, segment *m3u8.MediaSegment, block cipher.Block, iv []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, segment.URI, nil)
	if err != nil {
		return nil, err
	}
	req.WithContext(d.Context)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	blockMode := cipher.NewCBCDecrypter(block, iv[:block.BlockSize()])
	decrypted := make([]byte, len(raw))
	blockMode.CryptBlocks(decrypted, raw)
	raw = d.pkcs5UnPadding(decrypted)

	return raw, nil
}

// https://github.com/oopsguy/m3u8/blob/4150e93ec8f4f8718875a02973f5d792648ecb97/tool/crypt.go#L47
func (d Downloader) pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}
