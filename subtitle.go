package crunchyroll

import (
	"io"
)

type Subtitle struct {
	crunchy *Crunchyroll

	Locale LOCALE `json:"locale"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

func (s Subtitle) Save(writer io.Writer) error {
	resp, err := s.crunchy.Client.Get(s.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(writer, resp.Body)
	return err
}
