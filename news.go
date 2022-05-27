package crunchyroll

// News contains all information about a news.
type News struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Image       string `json:"image"`
	Creator     string `json:"creator"`
	PublishDate string `json:"publish_date"`
	Description string `json:"description"`
}

type TopNews struct {
	News
}

type LatestNews struct {
	News
}
