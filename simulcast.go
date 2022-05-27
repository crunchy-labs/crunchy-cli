package crunchyroll

// Simulcast contains all information about a simulcast season.
type Simulcast struct {
	ID string `json:"id"`

	Localization struct {
		Title string `json:"title"`

		// appears to be always an empty string.
		Description string `json:"description"`
	} `json:"localization"`
}
