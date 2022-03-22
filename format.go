package crunchyroll

import (
	"github.com/grafov/m3u8"
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

// Download downloads the Format with the via Downloader specified options
func (f *Format) Download(downloader Downloader) error {
	return downloader.download(f)
}
