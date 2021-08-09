package crunchyroll

import (
	"io"
	"os"
)

type Subtitle struct {
	crunchy *Crunchyroll

	Locale LOCALE `json:"locale"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

func (s Subtitle) Download(file *os.File) error {
	resp, err := s.crunchy.Client.Get(s.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}
