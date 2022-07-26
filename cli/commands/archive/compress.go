package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/crunchy-labs/crunchy-cli/utils"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Compress interface {
	io.Closer

	NewFile(information utils.FormatInformation) (io.WriteCloser, error)
}

func NewGzipCompress(file *os.File) *TarCompress {
	gw := gzip.NewWriter(file)
	return &TarCompress{
		parent: gw,
		dst:    tar.NewWriter(gw),
	}
}

func NewTarCompress(file *os.File) *TarCompress {
	return &TarCompress{
		dst: tar.NewWriter(file),
	}
}

type TarCompress struct {
	Compress

	wg sync.WaitGroup

	parent *gzip.Writer
	dst    *tar.Writer
}

func (tc *TarCompress) Close() error {
	// we have to wait here in case the actual content isn't copied completely into the
	// writer yet
	tc.wg.Wait()

	var err, err2 error
	if tc.parent != nil {
		err2 = tc.parent.Close()
	}
	err = tc.dst.Close()

	if err != nil && err2 != nil {
		// best way to show double errors at once that I've found
		return fmt.Errorf("%v\n%v", err, err2)
	} else if err == nil && err2 != nil {
		err = err2
	}

	return err
}

func (tc *TarCompress) NewFile(information utils.FormatInformation) (io.WriteCloser, error) {
	rp, wp := io.Pipe()
	go func() {
		tc.wg.Add(1)
		defer tc.wg.Done()
		var buf bytes.Buffer
		io.Copy(&buf, rp)

		header := &tar.Header{
			Name:     filepath.Join(fmt.Sprintf("S%2d", information.SeasonNumber), information.Title),
			ModTime:  time.Now(),
			Mode:     0644,
			Typeflag: tar.TypeReg,
			// fun fact: I did not set the size for quiet some time because I thought that it isn't
			// required. well because of this I debugged this part for multiple hours because without
			// proper size information only a tiny amount gets copied into the tar (or zip) writer.
			// this is also the reason why the file content is completely copied into a buffer before
			// writing it to the writer. I could bypass this and save some memory but this requires
			// some rewriting and im nearly at the (planned) finish for version 2 so nah in the future
			// maybe
			Size: int64(buf.Len()),
		}
		tc.dst.WriteHeader(header)
		io.Copy(tc.dst, &buf)
	}()
	return wp, nil
}

func NewZipCompress(file *os.File) *ZipCompress {
	return &ZipCompress{
		dst: zip.NewWriter(file),
	}
}

type ZipCompress struct {
	Compress

	wg sync.WaitGroup

	dst *zip.Writer
}

func (zc *ZipCompress) Close() error {
	zc.wg.Wait()
	return zc.dst.Close()
}

func (zc *ZipCompress) NewFile(information utils.FormatInformation) (io.WriteCloser, error) {
	rp, wp := io.Pipe()
	go func() {
		zc.wg.Add(1)
		defer zc.wg.Done()

		var buf bytes.Buffer
		io.Copy(&buf, rp)

		header := &zip.FileHeader{
			Name:               filepath.Join(fmt.Sprintf("S%2d", information.SeasonNumber), information.Title),
			Modified:           time.Now(),
			Method:             zip.Deflate,
			UncompressedSize64: uint64(buf.Len()),
		}
		header.SetMode(0644)

		hw, _ := zc.dst.CreateHeader(header)
		io.Copy(hw, &buf)
	}()

	return wp, nil
}
