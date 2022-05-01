package utils

import (
	"github.com/ByteDream/crunchyroll-go/v2"
)

// AllLocales is an array of all available locales.
var AllLocales = []crunchyroll.LOCALE{
	crunchyroll.JP,
	crunchyroll.US,
	crunchyroll.LA,
	crunchyroll.ES,
	crunchyroll.FR,
	crunchyroll.PT,
	crunchyroll.BR,
	crunchyroll.IT,
	crunchyroll.DE,
	crunchyroll.RU,
	crunchyroll.AR,
}

// ValidateLocale validates if the given locale actually exist.
func ValidateLocale(locale crunchyroll.LOCALE) bool {
	for _, l := range AllLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// LocaleLanguage returns the country by its locale.
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
	case crunchyroll.PT:
		return "Portuguese (Europe)"
	case crunchyroll.BR:
		return "Portuguese (Brazil)"
	case crunchyroll.IT:
		return "Italian"
	case crunchyroll.DE:
		return "German"
	case crunchyroll.RU:
		return "Russian"
	case crunchyroll.AR:
		return "Arabic"
	default:
		return ""
	}
}
