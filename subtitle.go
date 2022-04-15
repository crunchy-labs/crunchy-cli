package crunchyroll

import (
	"io"
	"net/http"
)

// Subtitle contains the information about a video subtitle.
type Subtitle struct {
	crunchy *Crunchyroll

	Locale LOCALE `json:"locale"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

// Save writes the subtitle to the given io.Writer.
func (s Subtitle) Save(writer io.Writer) error {
	req, err := http.NewRequestWithContext(s.crunchy.Context, http.MethodGet, s.URL, nil)
	if err != nil {
		return err
	}

	resp, err := s.crunchy.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(writer, resp.Body)
	return err
}
