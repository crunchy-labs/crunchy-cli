package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// BrowseSortType represents a sort type to sort Crunchyroll.Browse items after.
type BrowseSortType string

const (
	BrowseSortPopularity   BrowseSortType = "popularity"
	BrowseSortNewlyAdded                  = "newly_added"
	BrowseSortAlphabetical                = "alphabetical"
)

// BrowseOptions represents options for browsing the crunchyroll catalog.
type BrowseOptions struct {
	// Categories specifies the categories of the entries.
	Categories []string `param:"categories"`

	// IsDubbed specifies whether the entries should be dubbed.
	IsDubbed bool `param:"is_dubbed"`

	// IsSubbed specifies whether the entries should be subbed.
	IsSubbed bool `param:"is_subbed"`

	// Simulcast specifies a particular simulcast season by id in which the entries have been aired.
	Simulcast string `param:"season_tag"`

	// Sort specifies how the entries should be sorted.
	Sort BrowseSortType `param:"sort_by"`

	// Start specifies the index from which the entries should be returned.
	Start uint `param:"start"`

	// Type specifies the media type of the entries.
	Type MediaType `param:"type"`
}

// Browse browses the crunchyroll catalog filtered by the specified options and returns all found series and movies within the given limit.
func (c *Crunchyroll) Browse(options BrowseOptions, limit uint) (s []*Series, m []*Movie, err error) {
	query, err := encodeStructToQueryValues(options)
	if err != nil {
		return nil, nil, err
	}

	browseEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/browse?%s&n=%d&locale=%s",
		query, limit, c.Locale)
	resp, err := c.request(browseEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'browse' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		switch item.(map[string]interface{})["type"] {
		case "series":
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
		case "movie_listing":
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

// FindVideoByName finds a Video (Season or Movie) by its name.
// Use this in combination with ParseVideoURL and hand over the corresponding results
// to this function.
//
// Deprecated: Use Search instead. The first result sometimes isn't the correct one
// so this function is inaccurate in some cases.
// See https://github.com/ByteDream/crunchyroll-go/issues/22 for more information.
func (c *Crunchyroll) FindVideoByName(seriesName string) (Video, error) {
	s, m, err := c.Search(seriesName, 1)
	if err != nil {
		return nil, err
	}

	if len(s) > 0 {
		return s[0], nil
	} else if len(m) > 0 {
		return m[0], nil
	}
	return nil, fmt.Errorf("no series or movie could be found")
}

// FindEpisodeByName finds an episode by its crunchyroll series name and episode title.
// Use this in combination with ParseEpisodeURL and hand over the corresponding results
// to this function.
func (c *Crunchyroll) FindEpisodeByName(seriesName, episodeTitle string) ([]*Episode, error) {
	series, _, err := c.Search(seriesName, 5)
	if err != nil {
		return nil, err
	}

	var matchingEpisodes []*Episode
	for _, s := range series {
		seasons, err := s.Seasons()
		if err != nil {
			return nil, err
		}

		for _, season := range seasons {
			episodes, err := season.Episodes()
			if err != nil {
				return nil, err
			}
			for _, episode := range episodes {
				if episode.SlugTitle == episodeTitle {
					matchingEpisodes = append(matchingEpisodes, episode)
				}
			}
		}
	}

	return matchingEpisodes, nil
}

// Search searches a query and returns all found series and movies within the given limit.
func (c *Crunchyroll) Search(query string, limit uint) (s []*Series, m []*Movie, err error) {
	searchEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/search?q=%s&n=%d&type=&locale=%s",
		query, limit, c.Locale)
	resp, err := c.request(searchEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'search' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		item := item.(map[string]interface{})
		if item["total"].(float64) > 0 {
			switch item["type"] {
			case "series":
				for _, series := range item["items"].([]interface{}) {
					series2 := &Series{
						crunchy: c,
					}
					if err := decodeMapToStruct(series, series2); err != nil {
						return nil, nil, err
					}
					if err := decodeMapToStruct(series.(map[string]interface{})["series_metadata"].(map[string]interface{}), series2); err != nil {
						return nil, nil, err
					}

					s = append(s, series2)
				}
			case "movie_listing":
				for _, movie := range item["items"].([]interface{}) {
					movie2 := &Movie{
						crunchy: c,
					}
					if err := decodeMapToStruct(movie, movie2); err != nil {
						return nil, nil, err
					}

					m = append(m, movie2)
				}
			}
		}
	}

	return s, m, nil
}
