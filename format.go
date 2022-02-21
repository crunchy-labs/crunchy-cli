package crunchyroll

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/grafov/m3u8"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type FormatType string

const (
	EPISODE FormatType = "episodes"
	MOVIE              = "movies"
)

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

// InitVideo initializes the Format.Video completely.
// The Format.Video.Chunklist pointer is, by default, nil because an additional
// request must be made to receive its content. The request is not made when
// initializing a Format struct because it would probably cause an intense overhead
// since Format.Video.Chunklist is only used sometimes
func (f *Format) InitVideo() error {
	if f.Video.Chunklist == nil {
		resp, err := f.crunchy.Client.Get(f.Video.URI)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		playlist, _, err := m3u8.DecodeFrom(resp.Body, true)
		if err != nil {
			return err
		}
		f.Video.Chunklist = playlist.(*m3u8.MediaPlaylist)
	}
	return nil
}

func (f *Format) Download(downloader Downloader) error {
	if _, err := os.Stat(downloader.Filename); err == nil && !downloader.IgnoreExisting {
		return fmt.Errorf("file %s already exists", downloader.Filename)
	}
	if _, err := os.Stat(downloader.TempDir); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(downloader.TempDir, 0755)
		}
		if err != nil {
			return err
		}
	} else if !downloader.IgnoreExisting {
		content, err := os.ReadDir(downloader.TempDir)
		if err != nil {
			return err
		}
		if len(content) > 0 {
			return fmt.Errorf("directory %s is not empty", downloader.Filename)
		}
	}

	if downloader.DeleteTempAfter {
		defer os.RemoveAll(downloader.TempDir)
	}
	if err := download(downloader.Context, f, downloader.TempDir, downloader.Goroutines, downloader.LockOnSegmentDownload, downloader.OnSegmentDownload); err != nil {
		return err
	}

	if downloader.FFmpegOpts != nil {
		return mergeSegmentsFFmpeg(downloader.Context, downloader.TempDir, downloader.Filename, downloader.FFmpegOpts)
	} else {
		return mergeSegments(downloader.Context, downloader.TempDir, downloader.Filename)
	}
}

// mergeSegments reads every file in tempDir and writes their content to the outputFile.
// The given output file gets created or overwritten if already existing
func mergeSegments(context context.Context, tempDir string, outputFile string) error {
	dir, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
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
		select {
		case <-context.Done():
			return context.Err()
		default:
		}

		bodyAsBytes, err := ioutil.ReadFile(filepath.Join(tempDir, file.Name()))
		if err != nil {
			return err
		}
		if _, err = writer.Write(bodyAsBytes); err != nil {
			return err
		}
	}
	return nil
}

// mergeSegmentsFFmpeg reads every file in tempDir and merges their content to the outputFile
// with ffmpeg (https://ffmpeg.org/).
// The given output file gets created or overwritten if already existing
func mergeSegmentsFFmpeg(context context.Context, tempDir string, outputFile string, opts []string) error {
	dir, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(tempDir, "list.txt"))
	if err != nil {
		return err
	}
	// -1 is the list.txt file
	for i := 0; i < len(dir)-1; i++ {
		fmt.Fprintf(f, "file '%s.ts'\n", filepath.Join(tempDir, strconv.Itoa(i)))
	}
	f.Close()

	// predefined options ... custom options ... predefined output filename
	command := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", f.Name(),
		"-c", "copy",
	}
	if opts != nil {
		command = append(command, opts...)
	}
	command = append(command, outputFile)

	var errBuf bytes.Buffer
	cmd := exec.Command("ffmpeg",
		command...)
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return err
	}

	cmdChan := make(chan error, 1)
	go func() {
		cmdChan <- cmd.Wait()
	}()

	select {
	case err = <-cmdChan:
		if err != nil {
			if errBuf.Len() > 0 {
				return fmt.Errorf(errBuf.String())
			} else {
				return err
			}
		}
		return nil
	case <-context.Done():
		cmd.Process.Kill()
		return context.Err()
	}
}
