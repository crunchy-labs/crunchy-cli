package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Simulcasts returns all available simulcast seasons for the current locale.
func (c *Crunchyroll) Simulcasts() (s []*Simulcast, err error) {
	seasonListEndpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/season_list?locale=%s", c.Locale)
	resp, err := c.request(seasonListEndpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'season_list' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		simulcast := &Simulcast{}
		if err := decodeMapToStruct(item, simulcast); err != nil {
			return nil, err
		}

		s = append(s, simulcast)
	}

	return s, nil
}

// Simulcast contains all information about a simulcast season.
type Simulcast struct {
	ID string `json:"id"`

	Localization struct {
		Title string `json:"title"`

		// appears to be always an empty string.
		Description string `json:"description"`
	} `json:"localization"`
}
