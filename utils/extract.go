package utils

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go/v3"
	"github.com/ByteDream/crunchyroll-go/v3/utils"
	"regexp"
	"strconv"
	"strings"
)

var urlFilter = regexp.MustCompile(`(S(\d+))?(E(\d+))?((-)(S(\d+))?(E(\d+))?)?(,|$)`)

func ExtractEpisodes(url string, locales ...crunchyroll.LOCALE) ([][]*crunchyroll.Episode, error) {
	var matches [][]string

	lastOpen := strings.LastIndex(url, "[")
	if strings.HasSuffix(url, "]") && lastOpen != -1 && lastOpen < len(url) {
		matches = urlFilter.FindAllStringSubmatch(url[lastOpen+1:len(url)-1], -1)

		var all string
		for _, match := range matches {
			all += match[0]
		}
		if all != url[lastOpen+1:len(url)-1] {
			return nil, fmt.Errorf("invalid episode filter")
		}
		url = url[:lastOpen]
	}

	final := make([][]*crunchyroll.Episode, len(locales))
	episodes, err := Crunchy.ExtractEpisodesFromUrl(url, locales...)
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes: %v", err)
	}

	if len(episodes) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	if matches != nil {
		for _, match := range matches {
			fromSeason, fromEpisode, toSeason, toEpisode := -1, -1, -1, -1
			if match[2] != "" {
				fromSeason, _ = strconv.Atoi(match[2])
			}
			if match[4] != "" {
				fromEpisode, _ = strconv.Atoi(match[4])
			}
			if match[8] != "" {
				toSeason, _ = strconv.Atoi(match[8])
			}
			if match[10] != "" {
				toEpisode, _ = strconv.Atoi(match[10])
			}

			if match[6] != "-" {
				toSeason = fromSeason
				toEpisode = fromEpisode
			}

			tmpEps := make([]*crunchyroll.Episode, 0)
			for _, episode := range episodes {
				if fromSeason != -1 && (episode.SeasonNumber < fromSeason || (fromEpisode != -1 && episode.EpisodeNumber < fromEpisode)) {
					continue
				} else if fromSeason == -1 && fromEpisode != -1 && episode.EpisodeNumber < fromEpisode {
					continue
				} else if toSeason != -1 && (episode.SeasonNumber > toSeason || (toEpisode != -1 && episode.EpisodeNumber > toEpisode)) {
					continue
				} else if toSeason == -1 && toEpisode != -1 && episode.EpisodeNumber > toEpisode {
					continue
				} else {
					tmpEps = append(tmpEps, episode)
				}
			}

			if len(tmpEps) == 0 {
				return nil, fmt.Errorf("no episodes are matching the given filter")
			}

			episodes = tmpEps
		}
	}

	localeSorted, err := utils.SortEpisodesByAudio(episodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio locale: %v", err)
	}
	for i, locale := range locales {
		final[i] = append(final[i], localeSorted[locale]...)
	}

	return final, nil
}
