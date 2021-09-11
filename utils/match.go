package utils

import "regexp"

// MatchVideo tries to extract the crunchyroll series / movie name out of the given url
func MatchVideo(url string) (seriesName string, ok bool) {
	pattern := regexp.MustCompile(`(?m)^https?://(www\.)?crunchyroll\.com(/\w{2}(-\w{2})?)?/(?P<series>[^/]+)/?$`)
	if urlMatch := pattern.FindAllStringSubmatch(url, -1); len(urlMatch) != 0 {
		groups := regexGroups(urlMatch, pattern.SubexpNames()...)
		seriesName = groups["series"]

		if seriesName != "" {
			ok = true
		}
	}
	return
}

// MatchEpisode tries to extract the crunchyroll series name and title out of the given url
func MatchEpisode(url string) (seriesName, title string, ok bool) {
	pattern := regexp.MustCompile(`(?m)^https?://(www\.)?crunchyroll\.com(/\w{2}(-\w{2})?)?/(?P<series>[^/]+)/episode-\d+-(?P<title>\D+).*`)
	if urlMatch := pattern.FindAllStringSubmatch(url, -1); len(urlMatch) != 0 {
		groups := regexGroups(urlMatch, pattern.SubexpNames()...)
		seriesName = groups["series"]
		title = groups["title"]

		if seriesName != "" && title != "" {
			ok = true
		}
	}
	return
}

func regexGroups(parsed [][]string, subexpNames ...string) map[string]string {
	groups := map[string]string{}
	for _, match := range parsed {
		for i, content := range match {
			if subexpName := subexpNames[i]; subexpName != "" {
				groups[subexpName] = content
			}
		}
	}
	return groups
}
