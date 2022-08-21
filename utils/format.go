package utils

import (
	"fmt"
	"github.com/crunchy-labs/crunchyroll-go/v3"
	"reflect"
	"runtime"
	"strings"
)

type FormatInformation struct {
	// the Format to download
	Format *crunchyroll.Format

	// additional formats which are only used by archive.go
	AdditionalFormats []*crunchyroll.Format

	Title         string             `json:"title"`
	SeriesName    string             `json:"series_name"`
	SeasonName    string             `json:"season_name"`
	SeasonNumber  int                `json:"season_number"`
	EpisodeNumber int                `json:"episode_number"`
	Resolution    string             `json:"resolution"`
	FPS           float64            `json:"fps"`
	Audio         crunchyroll.LOCALE `json:"audio"`
	Subtitle      crunchyroll.LOCALE `json:"subtitle"`
}

func (fi FormatInformation) FormatString(source string) string {
	fields := reflect.TypeOf(fi)
	values := reflect.ValueOf(fi)

	for i := 0; i < fields.NumField(); i++ {
		var valueAsString string
		switch value := values.Field(i); value.Kind() {
		case reflect.String:
			valueAsString = value.String()
		case reflect.Int:
			valueAsString = fmt.Sprintf("%02d", value.Int())
		case reflect.Float64:
			valueAsString = fmt.Sprintf("%.2f", value.Float())
		case reflect.Bool:
			valueAsString = fields.Field(i).Tag.Get("json")
			if !value.Bool() {
				valueAsString = "no " + valueAsString
			}
		}

		if runtime.GOOS != "windows" {
			for _, char := range []string{"/"} {
				valueAsString = strings.ReplaceAll(valueAsString, char, "")
			}
		} else {
			for _, char := range []string{"\\", "<", ">", ":", "\"", "/", "|", "?", "*"} {
				valueAsString = strings.ReplaceAll(valueAsString, char, "")
			}
		}

		source = strings.ReplaceAll(source, "{"+fields.Field(i).Tag.Get("json")+"}", valueAsString)
	}

	return source
}
