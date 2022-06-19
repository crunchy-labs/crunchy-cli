package crunchyroll

type Image struct {
	Height int    `json:"height"`
	Source string `json:"source"`
	Type   string `json:"type"`
	Width  int    `json:"width"`
}

type Panel struct {
	Title            string `json:"title"`
	PromoTitle       string `json:"promo_title"`
	Slug             string `json:"slug"`
	Playback         string `json:"playback"`
	PromoDescription string `json:"promo_description"`
	Images           struct {
		Thumbnail  [][]Image `json:"thumbnail"`
		PosterTall [][]Image `json:"poster_tall"`
		PosterWide [][]Image `json:"poster_wide"`
	} `json:"images"`
	ID          string             `json:"id"`
	Description string             `json:"description"`
	ChannelID   string             `json:"channel_id"`
	Type        WatchlistEntryType `json:"type"`
	ExternalID  string             `json:"external_id"`
	SlugTitle   string             `json:"slug_title"`
	// not null if Type is WATCHLISTENTRYEPISODE
	EpisodeMetadata *Episode `json:"episode_metadata"`
	// not null if Type is WATCHLISTENTRYSERIES
	SeriesMetadata *Series `json:"series_metadata"`
}
