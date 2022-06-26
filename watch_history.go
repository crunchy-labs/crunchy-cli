package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WatchHistory returns the history of watched episodes based on the currently logged in account from the given page with the given size.
func (c *Crunchyroll) WatchHistory(page uint, size uint) (e []*HistoryEpisode, err error) {
	watchHistoryEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/watch-history/%s?page=%d&page_size=%d&locale=%s",
		c.Config.AccountID, page, size, c.Locale)
	resp, err := c.request(watchHistoryEndpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'watch-history' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		panel := item.(map[string]interface{})["panel"]

		episode := &Episode{
			crunchy: c,
		}
		if err := decodeMapToStruct(panel, episode); err != nil {
			return nil, err
		}

		historyEpisode := &HistoryEpisode{
			Episode: episode,
		}
		if err := decodeMapToStruct(item, historyEpisode); err != nil {
			return nil, err
		}

		e = append(e, historyEpisode)
	}

	return e, nil
}
