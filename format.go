package crunchyroll

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"github.com/grafov/m3u8"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	EPISODE FormatType = "episodes"
	MOVIE              = "movies"
)

type FormatType string
type Format struct {
	crunchy *Crunchyroll

	ID string
	// FormatType represents if the format parent is an episode or a movie
	FormatType  FormatType
	Video       *m3u8.Variant
	AudioLocale LOCALE
	Hardsub     LOCALE
	Subtitles   []*Subtitle
}

// Download calls DownloadGoroutines with 4 goroutines.
// See DownloadGoroutines for more details
//
// Deprecated: Use DownloadGoroutines instead
func (f *Format) Download(output *os.File, onSegmentDownload func(segment *m3u8.MediaSegment, current, total int, file *os.File, err error) error) error {
	return f.DownloadGoroutines(output, 4, func(segment *m3u8.MediaSegment, current, total int, file *os.File) error {
		return onSegmentDownload(segment, current, total, file, nil)
	})
}

// DownloadGoroutines downloads the format to the given output file (as .ts file).
// See Format.DownloadSegments for more information
func (f *Format) DownloadGoroutines(output *os.File, goroutines int, onSegmentDownload func(segment *m3u8.MediaSegment, current, total int, file *os.File) error) error {
	downloadDir, err := os.MkdirTemp("", "crunchy_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(downloadDir)

	if err := f.DownloadSegments(downloadDir, goroutines, onSegmentDownload); err != nil {
		return err
	}

	return f.mergeSegments(downloadDir, output)
}

// DownloadSegments downloads every mpeg transport stream segment to a given directory (more information below).
// After every segment download onSegmentDownload will be called with:
//		the downloaded segment, the current position, the total size of segments to download, the file where the segment content was written to an error (if occurred).
// The filename is always <number of downloaded segment>.ts
//
// Short explanation:
// 		The actual crunchyroll video is split up in multiple segments (or video files) which have to be downloaded and merged after to generate a single video file.
//		And this function just downloads each of this segment into the given directory.
// 		See https://en.wikipedia.org/wiki/MPEG_transport_stream for more information
func (f *Format) DownloadSegments(outputDir string, goroutines int, onSegmentDownload func(segment *m3u8.MediaSegment, current, total int, file *os.File) error) error {
	resp, err := f.crunchy.Client.Get(f.Video.URI)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// reads the m3u8 file
	playlist, _, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return err
	}
	// extracts the segments from the playlist
	var segments []*m3u8.MediaSegment
	for _, segment := range playlist.(*m3u8.MediaPlaylist).Segments {
		// some segments are nil, so they have to be filtered out
		if segment != nil {
			segments = append(segments, segment)
		}
	}

	var wg sync.WaitGroup
	chunkSize := int(math.Ceil(float64(len(segments)) / float64(goroutines)))

	// when a onSegmentDownload call returns an error, this channel will be set to true and stop all goroutines
	quit := make(chan bool)

	// receives the decrypt block and iv from the first segment.
	// in my tests, only the first segment has specified this data, so the decryption data from this first segments will be used in every other segment too
	block, iv, err := f.getCrypt(segments[0])
	if err != nil {
		return err
	}

	var total int32
	for i := 0; i < len(segments); i += chunkSize {
		wg.Add(1)
		end := i + chunkSize
		if end > len(segments) {
			end = len(segments)
		}
		i := i

		fmt.Println(i, end)

		go func() {
			for j, segment := range segments[i:end] {
				select {
				case <-quit:
					break
				default:
					var file *os.File
					k := 1
					for ; k < 4; k++ {
						file, err = f.downloadSegment(segment, filepath.Join(outputDir, fmt.Sprintf("%d.ts", i+j)), block, iv)
						if err == nil {
							break
						}
						// sleep if an error occurs. very useful because sometimes the connection times out
						time.Sleep(5 * time.Duration(k) * time.Second)
					}
					if k == 4 {
						quit <- true
						return
					}
					if onSegmentDownload != nil {
						if err = onSegmentDownload(segment, int(atomic.AddInt32(&total, 1)), len(segments), file); err != nil {
							quit <- true
							file.Close()
							return
						}
					}
					file.Close()
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()

	select {
	case <-quit:
		return err
	default:
		return nil
	}
}

// getCrypt extracts the key and iv of a m3u8 segment and converts it into a cipher.Block block and a iv byte sequence
func (f *Format) getCrypt(segment *m3u8.MediaSegment) (block cipher.Block, iv []byte, err error) {
	var resp *http.Response

	resp, err = f.crunchy.Client.Get(segment.Key.URI)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	key, err := ioutil.ReadAll(resp.Body)

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

// downloadSegment downloads a segments, decrypts it and names it after the given index
func (f *Format) downloadSegment(segment *m3u8.MediaSegment, filename string, block cipher.Block, iv []byte) (*os.File, error) {
	// every segment is aes-128 encrypted and has to be decrypted when downloaded
	content, err := decryptSegment(f.crunchy.Client, segment, block, iv)
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

// mergeSegments reads every file in tempPath and write their content to output
func (f *Format) mergeSegments(tempPath string, output *os.File) error {
	dir, err := os.ReadDir(tempPath)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(output)
	defer writer.Flush()

	// sort the directory files after their numeric names
	sort.Slice(dir, func(i, j int) bool {
		iNum, err := strconv.Atoi(strings.Split(dir[i].Name(), ".")[0])
		if err != nil {
			return false
		}
		jNum, err := strconv.Atoi(strings.Split(dir[j].Name(), ".")[0])
		if err != nil {
			return false
		}
		return iNum < jNum
	})

	for _, file := range dir {
		bodyAsBytes, err := ioutil.ReadFile(filepath.Join(tempPath, file.Name()))
		if err != nil {
			return err
		}
		if _, err = writer.Write(bodyAsBytes); err != nil {
			return err
		}
	}
	return nil
}
