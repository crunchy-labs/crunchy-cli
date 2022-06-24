package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Recommendations returns series and movie recommendations from crunchyroll based on the currently logged in account within the given limit.
func (c *Crunchyroll) Recommendations(limit uint) (s []*Series, m []*Movie, err error) {
	recommendationsEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/%s/recommendations?n=%d&locale=%s",
		c.Config.AccountID, limit, c.Locale)
	resp, err := c.request(recommendationsEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'recommendations' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		switch item.(map[string]interface{})["type"] {
		case MediaTypeSeries:
			series := &Series{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, series); err != nil {
				return nil, nil, err
			}
			if err := decodeMapToStruct(item.(map[string]interface{})["series_metadata"].(map[string]interface{}), series); err != nil {
				return nil, nil, err
			}

			s = append(s, series)
		case MediaTypeMovie:
			movie := &Movie{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, movie); err != nil {
				return nil, nil, err
			}

			m = append(m, movie)
		}
	}

	return s, m, nil
}

// UpNext returns the episodes that are up next based on the currently logged in account within the given limit.
func (c *Crunchyroll) UpNext(limit uint) (e []*Episode, err error) {
	upNextAccountEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/%s/up_next_account?n=%d&locale=%s",
		c.Config.AccountID, limit, c.Locale)
	resp, err := c.request(upNextAccountEndpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'up_next_account' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		panel := item.(map[string]interface{})["panel"]

		episode := &Episode{
			crunchy: c,
		}
		if err := decodeMapToStruct(panel, episode); err != nil {
			return nil, err
		}

		e = append(e, episode)
	}

	return e, nil
}
