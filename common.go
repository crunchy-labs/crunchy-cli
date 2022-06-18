package crunchyroll

type Panel struct {
	Title            string `json:"title"`
	PromoTitle       string `json:"promo_title"`
	Slug             string `json:"slug"`
	Playback         string `json:"playback"`
	PromoDescription string `json:"promo_description"`
	Images           struct {
		Thumbnail [][]struct {
			Height int    `json:"height"`
			Source string `json:"source"`
			Type   string `json:"type"`
			Width  int    `json:"width"`
		} `json:"thumbnail"`
		PosterTall [][]struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Type   string `json:"type"`
			Source string `json:"source"`
		} `json:"poster_tall"`
		PosterWide [][]struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Type   string `json:"type"`
			Source string `json:"source"`
		} `json:"poster_wide"`
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
