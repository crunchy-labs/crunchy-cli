package crunchyroll

import "fmt"

// Wallpaper contains a wallpaper name which can be set via Account.ChangeWallpaper.
type Wallpaper string

// TinyUrl returns the url to the wallpaper in low resolution.
func (w *Wallpaper) TinyUrl() string {
	return fmt.Sprintf("https://static.crunchyroll.com/assets/wallpaper/360x115/%s", *w)
}

// BigUrl returns the url to the wallpaper in high resolution.
func (w *Wallpaper) BigUrl() string {
	return fmt.Sprintf("https://static.crunchyroll.com/assets/wallpaper/1920x400/%s", *w)
}
