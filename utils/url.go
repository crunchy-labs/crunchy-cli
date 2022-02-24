package utils

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
)

// ExtractEpisodesFromUrl extracts all episodes from an url.
// If audio is not empty, the episodes gets filtered after the given locale
func ExtractEpisodesFromUrl(crunchy *crunchyroll.Crunchyroll, url string, audio ...crunchyroll.LOCALE) ([]*crunchyroll.Episode, error) {
	series, episodes, err := ParseUrl(crunchy, url)
	if err != nil {
		return nil, err
	}

	var eps []*crunchyroll.Episode

	if series != nil {
		seasons, err := series.Seasons()
		if err != nil {
			return nil, err
		}
		for _, season := range seasons {
			if audio != nil {
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
		return nil, fmt.Errorf("could not find any matching episode")
	}

	return eps, nil
}

// ParseUrl parses the given url into a series or episode.
// The returning episode is a slice because non-beta urls have the same episode with different languages
func ParseUrl(crunchy *crunchyroll.Crunchyroll, url string) (*crunchyroll.Series, []*crunchyroll.Episode, error) {
	if seriesId, ok := crunchyroll.ParseBetaSeriesURL(url); ok {
		series, err := crunchyroll.SeriesFromID(crunchy, seriesId)
		if err != nil {
			return nil, nil, err
		}
		return series, nil, nil
	} else if episodeId, ok := crunchyroll.ParseBetaEpisodeURL(url); ok {
		episode, err := crunchyroll.EpisodeFromID(crunchy, episodeId)
		if err != nil {
			return nil, nil, err
		}
		return nil, []*crunchyroll.Episode{episode}, nil
	} else if seriesName, ok := crunchyroll.ParseVideoURL(url); ok {
		video, err := crunchy.FindVideoByName(seriesName)
		if err != nil {
			return nil, nil, err
		}
		return video.(*crunchyroll.Series), nil, nil
	} else if seriesName, title, _, _, ok := crunchyroll.ParseEpisodeURL(url); ok {
		episodes, err := crunchy.FindEpisodeByName(seriesName, title)
		if err != nil {
			return nil, nil, err
		}
		return nil, episodes, nil
	} else {
		return nil, nil, fmt.Errorf("invalid url %s", url)
	}
}
