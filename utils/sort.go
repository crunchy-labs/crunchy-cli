package utils

import (
	"github.com/ByteDream/crunchyroll-go"
	"strconv"
	"strings"
)

// MovieListingsByDuration sorts movie listings by their duration
type MovieListingsByDuration []*crunchyroll.MovieListing

func (mlbd MovieListingsByDuration) Len() int {
	return len(mlbd)
}
func (mlbd MovieListingsByDuration) Swap(i, j int) {
	mlbd[i], mlbd[j] = mlbd[j], mlbd[i]
}
func (mlbd MovieListingsByDuration) Less(i, j int) bool {
	return mlbd[i].DurationMS < mlbd[j].DurationMS
}

// EpisodesByDuration sorts episodes by their duration
type EpisodesByDuration []*crunchyroll.Episode

func (ebd EpisodesByDuration) Len() int {
	return len(ebd)
}
func (ebd EpisodesByDuration) Swap(i, j int) {
	ebd[i], ebd[j] = ebd[j], ebd[i]
}
func (ebd EpisodesByDuration) Less(i, j int) bool {
	return ebd[i].DurationMS < ebd[j].DurationMS
}

// FormatsByResolution sorts formats after their resolution
type FormatsByResolution []*crunchyroll.Format

func (fbr FormatsByResolution) Len() int {
	return len(fbr)
}
func (fbr FormatsByResolution) Swap(i, j int) {
	fbr[i], fbr[j] = fbr[j], fbr[i]
}
func (fbr FormatsByResolution) Less(i, j int) bool {
	iSplitRes := strings.Split(fbr[i].Video.Resolution, "x")
	iResX, _ := strconv.Atoi(iSplitRes[0])
	iResY, _ := strconv.Atoi(iSplitRes[1])

	jSplitRes := strings.Split(fbr[j].Video.Resolution, "x")
	jResX, _ := strconv.Atoi(jSplitRes[0])
	jResY, _ := strconv.Atoi(jSplitRes[1])

	return iResX+iResY < jResX+jResY
}
