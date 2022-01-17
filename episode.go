package crunchyroll

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

type Episode struct {
	crunchy *Crunchyroll

	siteCache map[string]interface{}

	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`

	SeriesID        string `json:"series_id"`
	SeriesTitle     string `json:"series_title"`
	SeriesSlugTitle string `json:"series_slug_title"`

	SeasonID        string `json:"season_id"`
	SeasonTitle     string `json:"season_title"`
	SeasonSlugTitle string `json:"season_slug_title"`
	SeasonNumber    int    `json:"season_number"`

	Episode             string  `json:"episode"`
	EpisodeNumber       int     `json:"episode_number"`
	SequenceNumber      float64 `json:"sequence_number"`
	ProductionEpisodeID string  `json:"production_episode_id"`

	Title            string `json:"title"`
	SlugTitle        string `json:"slug_title"`
	Description      string `json:"description"`
	NextEpisodeID    string `json:"next_episode_id"`
	NextEpisodeTitle string `json:"next_episode_title"`

	HDFlag        bool `json:"hd_flag"`
	IsMature      bool `json:"is_mature"`
	MatureBlocked bool `json:"mature_blocked"`

	EpisodeAirDate time.Time `json:"episode_air_date"`

	IsSubbed       bool     `json:"is_subbed"`
	IsDubbed       bool     `json:"is_dubbed"`
	IsClip         bool     `json:"is_clip"`
	SeoTitle       string   `json:"seo_title"`
	SeoDescription string   `json:"seo_description"`
	SeasonTags     []string `json:"season_tags"`

	AvailableOffline bool   `json:"available_offline"`
	Slug             string `json:"slug"`

	Images struct {
		Thumbnail [][]struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Type   string `json:"type"`
			Source string `json:"source"`
		} `json:"thumbnail"`
	} `json:"images"`

	DurationMS    int    `json:"duration_ms"`
	IsPremiumOnly bool   `json:"is_premium_only"`
	ListingID     string `json:"listing_id"`

	SubtitleLocales []LOCALE `json:"subtitle_locales"`
	Playback        string   `json:"playback"`

	AvailabilityNotes string `json:"availability_notes"`

	StreamID string
}

// EpisodeFromID returns an episode by its api id
func EpisodeFromID(crunchy *Crunchyroll, id string) (*Episode, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/episodes/%s?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		crunchy.Config.CountryCode,
		crunchy.Config.MaturityRating,
		crunchy.Config.Channel,
		id,
		crunchy.Locale,
		crunchy.Config.Signature,
		crunchy.Config.Policy,
		crunchy.Config.KeyPairID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	episode := &Episode{
		crunchy: crunchy,
	}
	if err := decodeMapToStruct(jsonBody, episode); err != nil {
		return nil, err
	}
	if episode.Playback != "" {
		streamHref := jsonBody["__links__"].(map[string]interface{})["streams"].(map[string]interface{})["href"].(string)
		if match := regexp.MustCompile(`(?m)^/cms/v2/\S+videos/(\w+)/streams$`).FindAllStringSubmatch(streamHref, -1); len(match) > 0 {
			episode.StreamID = match[0][1]
		}
	}

	return episode, nil
}

// AudioLocale returns the audio locale of the episode.
// Every episode in a season (should) have the same audio locale,
// so if you want to get the audio locale of a season, just call this method on the first episode of the season.
// Otherwise, if you call this function on every episode it will cause a massive delay and redundant network
// overload since it calls an api endpoint every time
func (e *Episode) AudioLocale() (LOCALE, error) {
	resp, err := e.crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/videos/%s/streams?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		e.crunchy.Config.CountryCode,
		e.crunchy.Config.MaturityRating,
		e.crunchy.Config.Channel,
		e.StreamID,
		e.crunchy.Locale,
		e.crunchy.Config.Signature,
		e.crunchy.Config.Policy,
		e.crunchy.Config.KeyPairID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)
	e.siteCache = jsonBody

	return LOCALE(jsonBody["audio_locale"].(string)), nil
}

// Streams returns all streams which are available for the episode
func (e *Episode) Streams() ([]*Stream, error) {
	return fromVideoStreams(e.crunchy, fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/videos/%s/streams?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		e.crunchy.Config.CountryCode,
		e.crunchy.Config.MaturityRating,
		e.crunchy.Config.Channel,
		e.StreamID,
		e.crunchy.Locale,
		e.crunchy.Config.Signature,
		e.crunchy.Config.Policy,
		e.crunchy.Config.KeyPairID))
}
