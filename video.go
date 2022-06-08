package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type video struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`

	Description string `json:"description"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	SlugTitle   string `json:"slug_title"`

	Images struct {
		PosterTall [][]struct {
			Height int    `json:"height"`
			Source string `json:"source"`
			Type   string `json:"type"`
			Width  int    `json:"width"`
		} `json:"poster_tall"`
		PosterWide [][]struct {
			Height int    `json:"height"`
			Source string `json:"source"`
			Type   string `json:"type"`
			Width  int    `json:"width"`
		} `json:"poster_wide"`
	} `json:"images"`
}

// Video is the base for Movie and Season.
type Video interface{}

// Movie contains information about a movie.
type Movie struct {
	video
	Video

	crunchy *Crunchyroll

	children []*MovieListing

	// not generated when calling MovieFromID.
	MovieListingMetadata struct {
		AvailabilityNotes   string   `json:"availability_notes"`
		AvailableOffline    bool     `json:"available_offline"`
		DurationMS          int      `json:"duration_ms"`
		ExtendedDescription string   `json:"extended_description"`
		FirstMovieID        string   `json:"first_movie_id"`
		IsDubbed            bool     `json:"is_dubbed"`
		IsMature            bool     `json:"is_mature"`
		IsPremiumOnly       bool     `json:"is_premium_only"`
		IsSubbed            bool     `json:"is_subbed"`
		MatureRatings       []string `json:"mature_ratings"`
		MovieReleaseYear    int      `json:"movie_release_year"`
		SubtitleLocales     []LOCALE `json:"subtitle_locales"`
	} `json:"movie_listing_metadata"`

	Playback string `json:"playback"`

	PromoDescription string `json:"promo_description"`
	PromoTitle       string `json:"promo_title"`
	SearchMetadata   struct {
		Score float64 `json:"score"`
	}
}

// MovieFromID returns a movie by its api id.
func MovieFromID(crunchy *Crunchyroll, id string) (*Movie, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/movies/%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
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

	movieListing := &Movie{
		crunchy: crunchy,
	}
	movieListing.ID = id
	if err = decodeMapToStruct(jsonBody, movieListing); err != nil {
		return nil, err
	}

	return movieListing, nil
}

// MovieListing returns all videos corresponding with the movie.
func (m *Movie) MovieListing() (movieListings []*MovieListing, err error) {
	if m.children != nil {
		return m.children, nil
	}

	resp, err := m.crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/movies?movie_listing_id=%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		m.crunchy.Config.CountryCode,
		m.crunchy.Config.MaturityRating,
		m.crunchy.Config.Channel,
		m.ID,
		m.crunchy.Locale,
		m.crunchy.Config.Signature,
		m.crunchy.Config.Policy,
		m.crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	for _, item := range jsonBody["items"].([]interface{}) {
		movieListing := &MovieListing{
			crunchy: m.crunchy,
		}
		if err = decodeMapToStruct(item, movieListing); err != nil {
			return nil, err
		}
		movieListings = append(movieListings, movieListing)
	}

	if m.crunchy.cache {
		m.children = movieListings
	}
	return movieListings, nil
}

// Series contains information about an anime series.
type Series struct {
	video
	Video

	crunchy *Crunchyroll

	children []*Season

	PromoDescription string `json:"promo_description"`
	PromoTitle       string `json:"promo_title"`

	AvailabilityNotes   string   `json:"availability_notes"`
	EpisodeCount        int      `json:"episode_count"`
	ExtendedDescription string   `json:"extended_description"`
	IsDubbed            bool     `json:"is_dubbed"`
	IsMature            bool     `json:"is_mature"`
	IsSimulcast         bool     `json:"is_simulcast"`
	IsSubbed            bool     `json:"is_subbed"`
	MatureBlocked       bool     `json:"mature_blocked"`
	MatureRatings       []string `json:"mature_ratings"`
	SeasonCount         int      `json:"season_count"`

	// not generated when calling SeriesFromID.
	SearchMetadata struct {
		Score float64 `json:"score"`
	}
}

// SeriesFromID returns a series by its api id.
func SeriesFromID(crunchy *Crunchyroll, id string) (*Series, error) {
	resp, err := crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/movies?movie_listing_id=%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
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

	series := &Series{
		crunchy: crunchy,
	}
	series.ID = id
	if err = decodeMapToStruct(jsonBody, series); err != nil {
		return nil, err
	}

	return series, nil
}

// Seasons returns all seasons of a series.
func (s *Series) Seasons() (seasons []*Season, err error) {
	if s.children != nil {
		return s.children, nil
	}

	resp, err := s.crunchy.request(fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/seasons?series_id=%s&locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		s.crunchy.Config.CountryCode,
		s.crunchy.Config.MaturityRating,
		s.crunchy.Config.Channel,
		s.ID,
		s.crunchy.Locale,
		s.crunchy.Config.Signature,
		s.crunchy.Config.Policy,
		s.crunchy.Config.KeyPairID), http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	for _, item := range jsonBody["items"].([]interface{}) {
		season := &Season{
			crunchy: s.crunchy,
		}
		if err = decodeMapToStruct(item, season); err != nil {
			return nil, err
		}
		seasons = append(seasons, season)
	}

	if s.crunchy.cache {
		s.children = seasons
	}
	return
}
