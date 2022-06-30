package utils

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go/v3"
	"github.com/ByteDream/crunchyroll-go/v3/utils"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// SystemLocale receives the system locale
// https://stackoverflow.com/questions/51829386/golang-get-system-language/51831590#51831590
func SystemLocale(verbose bool) crunchyroll.LOCALE {
	if runtime.GOOS != "windows" {
		if lang, ok := os.LookupEnv("LANG"); ok {
			var l crunchyroll.LOCALE
			if preSuffix := strings.Split(strings.Split(lang, ".")[0], "_"); len(preSuffix) == 1 {
				l = crunchyroll.LOCALE(preSuffix[0])
			} else {
				prefix := strings.Split(lang, "_")[0]
				l = crunchyroll.LOCALE(fmt.Sprintf("%s-%s", prefix, preSuffix[1]))
			}
			if !utils.ValidateLocale(l) {
				if verbose {
					Log.Err("%s is not a supported locale, using %s as fallback", l, crunchyroll.US)
				}
				l = crunchyroll.US
			}
			return l
		}
	} else {
		cmd := exec.Command("powershell", "Get-Culture | select -exp Name")
		if output, err := cmd.Output(); err == nil {
			l := crunchyroll.LOCALE(strings.Trim(string(output), "\r\n"))
			if !utils.ValidateLocale(l) {
				if verbose {
					Log.Err("%s is not a supported locale, using %s as fallback", l, crunchyroll.US)
				}
				l = crunchyroll.US
			}
			return l
		}
	}
	if verbose {
		Log.Err("Failed to get locale, using %s", crunchyroll.US)
	}
	return crunchyroll.US
}

func LocalesAsStrings() (locales []string) {
	for _, locale := range utils.AllLocales {
		locales = append(locales, string(locale))
	}
	sort.Strings(locales)
	return
}
