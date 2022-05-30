package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// MovieListing contains information about something which is called
// movie listing. I don't know what this means thb.
type MovieListing struct {
	crunchy *Crunchyroll

	ID string `json:"id"`

	Title       string `json:"title"`
	Slug        string `json:"slug"`
	SlugTitle   string `json:"slug_title"`
	Description string `json:"description"`

	Images struct {
		Thumbnail [][]struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Type   string `json:"type"`
			Source string `json:"source"`
		} `json:"thumbnail"`
	} `json:"images"`

	DurationMS       int    `json:"duration_ms"`
	IsPremiumOnly    bool   `json:"is_premium_only"`
	ListeningID      string `json:"listening_id"`
	IsMature         bool   `json:"is_mature"`
	AvailableOffline bool   `json:"available_offline"`
	IsSubbed         bool   `json:"is_subbed"`
	IsDubbed         bool   `json:"is_dubbed"`

	Playback          string `json:"playback"`
	AvailabilityNotes string `json:"availability_notes"`
}

// MovieListingFromID returns a movie listing by its api id.
func MovieListingFromID(crunchy *Crunchyroll, id string) (*MovieListing, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/movie_listing/%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		crunchy.Config.CountryCode,
		crunchy.Config.MaturityRating,
		crunchy.Config.Channel,
		id,
		crunchy.Locale,
		crunchy.Config.Signature,
		crunchy.Config.Policy,
		crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	movieListing := &MovieListing{
		crunchy: crunchy,
		ID:      id,
	}
	if err = decodeMapToStruct(jsonBody, movieListing); err != nil {
		return nil, err
	}

	return movieListing, nil
}

// AudioLocale is same as Episode.AudioLocale.
func (ml *MovieListing) AudioLocale() (LOCALE, error) {
	resp, err := ml.crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/videos/%s/streams?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		ml.crunchy.Config.CountryCode,
		ml.crunchy.Config.MaturityRating,
		ml.crunchy.Config.Channel,
		ml.ID,
		ml.crunchy.Locale,
		ml.crunchy.Config.Signature,
		ml.crunchy.Config.Policy,
		ml.crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	return LOCALE(jsonBody["audio_locale"].(string)), nil
}

// Streams returns all streams which are available for the movie listing.
func (ml *MovieListing) Streams() ([]*Stream, error) {
	return fromVideoStreams(ml.crunchy, fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/videos/%s/streams?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		ml.crunchy.Config.CountryCode,
		ml.crunchy.Config.MaturityRating,
		ml.crunchy.Config.Channel,
		ml.ID,
		ml.crunchy.Locale,
		ml.crunchy.Config.Signature,
		ml.crunchy.Config.Policy,
		ml.crunchy.Config.KeyPairID))
}
