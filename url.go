package crunchyroll

import (
	"fmt"
)

// ExtractEpisodesFromUrl extracts all episodes from an url.
// If audio is not empty, the episodes gets filtered after the given locale.
func (c *Crunchyroll) ExtractEpisodesFromUrl(url string, audio ...LOCALE) ([]*Episode, error) {
	series, episodes, err := c.ParseUrl(url)
	if err != nil {
		return nil, err
	}

	var eps []*Episode
	var notAvailableContinue bool

	if series != nil {
		seasons, err := series.Seasons()
		if err != nil {
			return nil, err
		}
		for _, season := range seasons {
			if audio != nil {
				if available, err := season.Available(); err != nil {
					return nil, err
				} else if !available {
					notAvailableContinue = true
					continue
				}

				locale, err := season.AudioLocale()
				if err != nil {
					return nil, err
				}

				var found bool
				for _, l := range audio {
					if locale == l {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			e, err := season.Episodes()
			if err != nil {
				return nil, err
			}
			eps = append(eps, e...)
		}
	} else if episodes != nil {
		if audio == nil {
			return episodes, nil
		}

		for _, episode := range episodes {
			// if no episode streams are available, calling episode.AudioLocale
			// will result in an unwanted error
			if !episode.Available() {
				notAvailableContinue = true
				continue
			}
			locale, err := episode.AudioLocale()
			if err != nil {
				return nil, err
			}
			if audio != nil {
				var found bool
				for _, l := range audio {
					if locale == l {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			eps = append(eps, episode)
		}
	}

	if len(eps) == 0 {
		if notAvailableContinue {
			return nil, fmt.Errorf("could not find any matching episode which is accessable with an non-premium account")
		} else {
			return nil, fmt.Errorf("could not find any matching episode")
		}
	}

	return eps, nil
}

// ParseUrl parses the given url into a series or episode.
// The returning episode is a slice because non-beta urls have the same episode with different languages.
func (c *Crunchyroll) ParseUrl(url string) (*Series, []*Episode, error) {
	if seriesId, ok := ParseBetaSeriesURL(url); ok {
		series, err := SeriesFromID(c, seriesId)
		if err != nil {
			return nil, nil, err
		}
		return series, nil, nil
	} else if episodeId, ok := ParseBetaEpisodeURL(url); ok {
		episode, err := EpisodeFromID(c, episodeId)
		if err != nil {
			return nil, nil, err
		}
		return nil, []*Episode{episode}, nil
	} else if seriesName, ok := ParseVideoURL(url); ok {
		video, err := c.FindVideoByName(seriesName)
		if err != nil {
			return nil, nil, err
		}
		return video.(*Series), nil, nil
	} else if seriesName, title, _, _, ok := ParseEpisodeURL(url); ok {
		episodes, err := c.FindEpisodeByName(seriesName, title)
		if err != nil {
			return nil, nil, err
		}
		return nil, episodes, nil
	} else {
		return nil, nil, fmt.Errorf("invalid url %s", url)
	}
}
