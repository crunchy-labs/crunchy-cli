package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// News returns the top and latest news from crunchyroll for the current locale within the given limits.
func (c *Crunchyroll) News(topLimit uint, latestLimit uint) (t []*News, l []*News, err error) {
	newsFeedEndpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/news_feed?top_news_n=%d&latest_news_n=%d&locale=%s",
		topLimit, latestLimit, c.Locale)
	resp, err := c.request(newsFeedEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'news_feed' response: %w", err)
	}

	topNews := jsonBody["top_news"].(map[string]interface{})
	for _, item := range topNews["items"].([]interface{}) {
		topNews := &News{}
		if err := decodeMapToStruct(item, topNews); err != nil {
			return nil, nil, err
		}

		t = append(t, topNews)
	}

	latestNews := jsonBody["latest_news"].(map[string]interface{})
	for _, item := range latestNews["items"].([]interface{}) {
		latestNews := &News{}
		if err := decodeMapToStruct(item, latestNews); err != nil {
			return nil, nil, err
		}

		l = append(l, latestNews)
	}

	return t, l, nil
}

// News contains all information about news.
type News struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Image       string `json:"image"`
	Creator     string `json:"creator"`
	PublishDate string `json:"publish_date"`
	Description string `json:"description"`
}
