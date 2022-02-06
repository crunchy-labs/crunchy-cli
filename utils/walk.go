package utils

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
)

// EpisodeWalker is an easy-to-use struct which walks down given urls
// and triggers functions where you can perform further actions with
// the delivered objects
type EpisodeWalker struct {
	// The Crunchyroll instance to perform all actions on
	Crunchyroll *crunchyroll.Crunchyroll

	// If CheckDuplicates is true, duplicated urls, seasons and episodes
	// are filtered out and the values given in OnSeason for example is
	// always unique
	CheckDuplicates bool

	// OnUrl gets called when an url is parsed.
	// The error is generally only nil when the url is invalid
	OnUrl func(url string, err error) error
	// OnSeason gets called when a season was parsed
	OnSeason func(season *crunchyroll.Season, err error) error
	// OnEpisode gets called when a season was parsed
	OnEpisode func(episode *crunchyroll.Episode, err error) error
}

// WalkURLs walks through all urls.
// Urls to seasons and episodes are support, normal as well as beta urls
func (ew EpisodeWalker) WalkURLs(urls []string) error {
	var episodeIds, seasonIds, seriesNames []string
	episodeNames := make(map[string][]string)

	for _, url := range urls {
		if episodeId, ok := crunchyroll.ParseBetaEpisodeURL(url); ok && !(ew.CheckDuplicates && sliceContains(episodeIds, episodeId)) {
			episodeIds = append(episodeIds, episodeId)
		} else if seasonId, ok := crunchyroll.ParseBetaSeriesURL(url); ok && !(ew.CheckDuplicates && sliceContains(seasonIds, seasonId)) {
			seasonIds = append(seasonIds, seasonId)
		} else if seriesName, title, _, _, ok := crunchyroll.ParseEpisodeURL(url); ok {
			if eps, ok := episodeNames[seriesName]; ok {
				if !ew.CheckDuplicates || !sliceContains(eps, title) {
					eps = append(eps, title)
				}
			} else {
				episodeNames[seriesName] = []string{title}
			}
		} else if seriesName, ok := crunchyroll.ParseVideoURL(url); ok && !(ew.CheckDuplicates && sliceContains(seriesNames, seriesName)) {
			seriesNames = append(seriesNames, seriesName)
		} else {
			err := fmt.Errorf("invalid url %s", url)
			if ew.OnUrl != nil {
				if err = ew.OnUrl(url, err); err != nil {
					return err
				}
				continue
			} else {
				return err
			}
		}

		if ew.OnUrl != nil {
			if err := ew.OnUrl(url, nil); err != nil {
				return err
			}
		}
	}

	for _, name := range seriesNames {
		video, err := ew.Crunchyroll.FindVideoByName(name)
		if err != nil {
			return err
		}
		// in all cases i've ever tested video was a series - even
		// if it was listed as a movie on the crunchyroll website.
		// i just hope no error occurs here :)
		seasons, err := video.(*crunchyroll.Series).Seasons()
		if err != nil {
			return err
		}
		for _, season := range seasons {
			if ew.CheckDuplicates {
				if sliceContains(seasonIds, season.ID) {
					continue
				}
				seasonIds = append(seasonIds, season.ID)
			}
			if ew.OnSeason != nil {
				if err := ew.OnSeason(season, nil); err != nil {
					return err
				}
			}
		}
	}

	if err := ew.walkEpisodeIds(episodeIds); err != nil {
		return err
	} else if err := ew.walkSeasonIds(seasonIds); err != nil {
		return err
	} else if err := ew.walkEpisodeNames(episodeNames); err != nil {
		return err
	}
	return nil
}

func (ew EpisodeWalker) walkEpisodeIds(episodeIds []string) error {
	var episodeIdsCheck []string

	for _, id := range episodeIds {
		if ew.CheckDuplicates {
			if sliceContains(episodeIdsCheck, id) {
				continue
			}
			episodeIdsCheck = append(episodeIdsCheck, id)
		}

		episode, err := crunchyroll.EpisodeFromID(ew.Crunchyroll, id)
		if ew.OnEpisode != nil {
			if err = ew.OnEpisode(episode, err); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ew EpisodeWalker) walkSeasonIds(seasonIds []string) error {
	var episodeIdsCheck []string

	for _, id := range seasonIds {
		season, err := crunchyroll.SeasonFromID(ew.Crunchyroll, id)
		if ew.OnSeason != nil {
			if err = ew.OnSeason(season, err); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		eps, err := season.Episodes()
		if err != nil {
			return err
		}
		for _, ep := range eps {
			if ew.CheckDuplicates {
				if sliceContains(episodeIdsCheck, ep.ID) {
					continue
				}
				episodeIdsCheck = append(episodeIdsCheck, ep.ID)
			}

			if ew.OnEpisode != nil {
				if err = ew.OnEpisode(ep, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (ew EpisodeWalker) walkEpisodeNames(episodeNames map[string][]string) error {
	var episodeIdsCheck []string

	for seriesName, episodeName := range episodeNames {
		video, err := ew.Crunchyroll.FindVideoByName(seriesName)
		if err != nil {
			return err
		}
		series := video.(*crunchyroll.Series)
		seasons, err := series.Seasons()
		if err != nil {
			return err
		}
		for _, season := range seasons {
			eps, err := season.Episodes()
			if err != nil {
				return err
			}
			for _, ep := range eps {
				for _, name := range episodeName {
					if ep.SlugTitle == name {
						if ew.OnEpisode != nil {
							if ew.CheckDuplicates {
								if sliceContains(episodeIdsCheck, ep.ID) {
									continue
								}
								episodeIdsCheck = append(episodeIdsCheck, ep.ID)
							}

							if err = ew.OnEpisode(ep, nil); err != nil {
								return err
							}
						}
						break
					}
				}
			}
		}
	}
	return nil
}

func sliceContains(slice []string, elem string) bool {
	for _, s := range slice {
		if elem == s {
			return true
		}
	}
	return false
}
