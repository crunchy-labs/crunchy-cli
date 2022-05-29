package crunchyroll

// Category contains all information about a category.
type Category struct {
	Category string `json:"tenant_category"`

	SubCategories []struct {
		Category       string `json:"tenant_category"`
		ParentCategory string `json:"parent_category"`

		Localization struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Locale      LOCALE `json:"locale"`
		} `json:"localization"`

		Slug string `json:"slug"`
	} `json:"sub_categories"`

	Images struct {
		Background []struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Type   string `json:"type"`
			Source string `json:"source"`
		} `json:"background"`

		Low []struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Type   string `json:"type"`
			Source string `json:"source"`
		} `json:"low"`
	} `json:"images"`

	Localization struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Locale      LOCALE `json:"locale"`
	} `json:"localization"`

	Slug string `json:"slug"`
}
