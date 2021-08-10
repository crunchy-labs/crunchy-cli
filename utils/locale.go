package utils

import (
	"github.com/ByteDream/crunchyroll-go"
)

var AllLocales = []crunchyroll.LOCALE{
	crunchyroll.JP,
	crunchyroll.US,
	crunchyroll.LA,
	crunchyroll.ES,
	crunchyroll.FR,
	crunchyroll.BR,
	crunchyroll.IT,
	crunchyroll.DE,
	crunchyroll.RU,
	crunchyroll.ME,
}

// ValidateLocale validates if the given locale actually exist
func ValidateLocale(locale crunchyroll.LOCALE) bool {
	for _, l := range AllLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// LocaleLanguage returns the country by its locale
func LocaleLanguage(locale crunchyroll.LOCALE) string {
	switch locale {
	case crunchyroll.JP:
		return "Japanese"
	case crunchyroll.US:
		return "English (US)"
	case crunchyroll.LA:
		return "Spanish (Latin America)"
	case crunchyroll.ES:
		return "Spanish (Spain)"
	case crunchyroll.FR:
		return "French"
	case crunchyroll.BR:
		return "Portuguese (Brazil)"
	case crunchyroll.IT:
		return "Italian"
	case crunchyroll.DE:
		return "German"
	case crunchyroll.RU:
		return "Russian"
	case crunchyroll.ME:
		return "Arabic"
	default:
		return ""
	}
}

// SubtitleByLocale returns the subtitle of a crunchyroll.Format by its locale.
// Check the second ok return value if the format has this subtitle
func SubtitleByLocale(format *crunchyroll.Format, locale crunchyroll.LOCALE) (subtitle *crunchyroll.Subtitle, ok bool) {
	if format.Subtitles == nil {
		return
	}
	for _, sub := range format.Subtitles {
		if sub.Locale == locale {
			return sub, true
		}
	}
	return
}
