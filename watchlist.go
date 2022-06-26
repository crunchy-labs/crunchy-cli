package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// WatchlistLanguageType represents a filter type to filter Crunchyroll.Watchlist entries after sub or dub.
type WatchlistLanguageType int

const (
	WatchlistLanguageSubbed WatchlistLanguageType = iota + 1
	WatchlistLanguageDubbed
)

type WatchlistOrderType string

const (
	WatchlistOrderAsc  = "asc"
	WatchlistOrderDesc = "desc"
)

// WatchlistOptions represents options for receiving the user watchlist.
type WatchlistOptions struct {
	// Order specified whether the results should be order ascending or descending.
	Order WatchlistOrderType

	// OnlyFavorites specifies whether only episodes which are marked as favorite should be returned.
	OnlyFavorites bool

	// LanguageType specifies whether returning episodes should be only subbed or dubbed.
	LanguageType WatchlistLanguageType

	// ContentType specified whether returning videos should only be series episodes or movies.
	// But tbh all movies I've searched on crunchy were flagged as series too, so this
	// parameter is kinda useless.
	ContentType MediaType
}

// Watchlist returns the watchlist entries for the currently logged in user.
func (c *Crunchyroll) Watchlist(options WatchlistOptions, limit uint) ([]*WatchlistEntry, error) {
	values := url.Values{}
	if options.Order == "" {
		options.Order = WatchlistOrderDesc
	}
	values.Set("order", string(options.Order))
	if options.OnlyFavorites {
		values.Set("only_favorites", "true")
	}
	switch options.LanguageType {
	case WatchlistLanguageSubbed:
		values.Set("is_subbed", "true")
	case WatchlistLanguageDubbed:
		values.Set("is_dubbed", "true")
	}
	values.Set("n", strconv.Itoa(int(limit)))
	values.Set("locale", string(c.Locale))

	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/%s/watchlist?%s", c.Config.AccountID, values.Encode())
	resp, err := c.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	var watchlistEntries []*WatchlistEntry
	if err := decodeMapToStruct(jsonBody["items"], &watchlistEntries); err != nil {
		return nil, err
	}

	for _, entry := range watchlistEntries {
		switch entry.Panel.Type {
		case WatchlistEntryEpisode:
			entry.Panel.EpisodeMetadata.crunchy = c
		case WatchlistEntrySeries:
			entry.Panel.SeriesMetadata.crunchy = c
		}
	}

	return watchlistEntries, nil
}

// WatchlistEntry contains information about an entry on the watchlist.
type WatchlistEntry struct {
	Panel Panel `json:"panel"`

	New            bool `json:"new"`
	NewContent     bool `json:"new_content"`
	IsFavorite     bool `json:"is_favorite"`
	NeverWatched   bool `json:"never_watched"`
	CompleteStatus bool `json:"complete_status"`
	Playahead      uint `json:"playahead"`
}
