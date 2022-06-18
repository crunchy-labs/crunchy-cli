package crunchyroll

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
