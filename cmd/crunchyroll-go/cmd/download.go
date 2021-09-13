package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/ByteDream/crunchyroll-go/utils"
	"github.com/grafov/m3u8"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var (
	audioFlag     string
	subtitleFlag  string
	noHardsubFlag bool

	directoryFlag string
	outputFlag    string

	resolutionFlag string

	alternativeProgressFlag bool
)

var getCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a video",
	Args:  cobra.MinimumNArgs(1),

	Run: func(cmd *cobra.Command, args []string) {
		loadCrunchy()

		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sig
			if cleanupPath != "" {
				os.RemoveAll(cleanupPath)
			}
			os.Exit(1)
		}()

		download(args)
	},
}

var (
	invalidWindowsChars = []string{"<", ">", ":", "\"", "/", "|", "\\", "?", "*"}
	invalidLinuxChars   = []string{"/"}
)

var cleanupPath string

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&audioFlag, "audio", "a", "", "The locale of the audio. Available locales: "+strings.Join(allLocalesAsStrings(), ", "))
	getCmd.Flags().StringVarP(&subtitleFlag, "subtitle", "s", "", "The locale of the subtitle. Available locales: "+strings.Join(allLocalesAsStrings(), ", "))
	getCmd.Flags().BoolVar(&noHardsubFlag, "no-hardsub", false, "Same as '-s', but the subtitles are not stored in the video itself, but in a separate file")

	cwd, _ := os.Getwd()
	getCmd.Flags().StringVarP(&directoryFlag, "directory", "d", cwd, "The directory to download the file to")
	getCmd.Flags().StringVarP(&outputFlag, "output", "o", "{title}.ts", "Name of the output file\n"+
		"If you use the following things in the name, the will get replaced\n"+
		"\t{title} » Title of the video\n"+
		"\t{season_title} » Title of the season\n"+
		"\t{season_number} » Number of the season\n"+
		"\t{episode_number} » Number of the episode\n"+
		"\t{resolution} » Resolution of the video\n"+
		"\t{fps} » Frame Rate of the video\n"+
		"\t{audio} » Audio locale of the video\n"+
		"\t{subtitle} » Subtitle locale of the video\n")

	getCmd.Flags().StringVarP(&resolutionFlag, "resolution", "r", "best", "The video resolution. Can either be specified via the pixels, the abbreviation for pixels, or 'common-use' words\n"+
		"\tAvailable pixels: 1920x1080, 1280x720, 640x480, 480x360, 426x240\n"+
		"\tAvailable abbreviations: 1080p, 720p, 480p, 360p, 240p\n"+
		"\tAvailable common-use words: best (best available resolution), worst (worst available resolution)\n")

	getCmd.Flags().BoolVar(&alternativeProgressFlag, "alternative-progress", false, "Shows an alternative, not so user-friendly progress instead of the progress bar")
}

type episodeInformation struct {
	Format      *crunchyroll.Format
	Title       string
	URL         string
	SeriesTitle string
	SeasonNum   int
	EpisodeNum  int
}

type information struct {
	Title         string             `json:"title"`
	SeriesName    string             `json:"series_name"`
	SeasonNumber  int                `json:"season_number"`
	EpisodeNumber int                `json:"episode_number"`
	OriginalURL   string             `json:"original_url"`
	DownloadURL   string             `json:"download_url"`
	Resolution    string             `json:"resolution"`
	FPS           float64            `json:"fps"`
	Audio         crunchyroll.LOCALE `json:"audio"`
	Subtitle      crunchyroll.LOCALE `json:"subtitle"`
	Hardsub       bool               `json:"hardsub"`
}

func download(urls []string) {
	if path.Ext(outputFlag) != ".ts" && !hasFFmpeg() {
		out.Fatalf("The file ending for the output file (%s) is not `.ts`. "+
			"Install ffmpeg (https://ffmpeg.org/download.html) use other media file endings (e.g. `.mp4`)\n", outputFlag)
	}
	allEpisodes, total, successes := parseURLs(urls)
	out.Infof("%d of %d episodes could be parsed\n", successes, total)

	out.Empty()
	if len(allEpisodes) == 0 {
		out.Fatalf("Nothing to download, aborting\n")
	}
	out.Infof("Downloads:")
	for i, episode := range allEpisodes {
		video := episode.Format.Video
		out.Infof("\t%d. %s » %spx, %.2f FPS, %s audio (%s S%02dE%02d)\n",
			i+1, episode.Title, video.Resolution, video.FrameRate, utils.LocaleLanguage(episode.Format.AudioLocale), episode.SeriesTitle, episode.SeasonNum, episode.EpisodeNum)
	}

	if fileInfo, stat := os.Stat(directoryFlag); os.IsNotExist(stat) {
		if err := os.MkdirAll(directoryFlag, 0777); err != nil {
			out.Fatalf("Failed to create directory which was given from the `-d`/`--directory` flag: %s\n", err)
		}
	} else if !fileInfo.IsDir() {
		out.Fatalf("%s (given from the `-d`/`--directory` flag) is not a directory\n", directoryFlag)
	}

	var success int
	for _, episode := range allEpisodes {
		var subtitle crunchyroll.LOCALE
		if subtitleFlag != "" {
			subtitle = localeToLOCALE(subtitleFlag)
		}
		info := information{
			Title:         episode.Title,
			SeriesName:    episode.SeriesTitle,
			SeasonNumber:  episode.SeasonNum,
			EpisodeNumber: episode.EpisodeNum,
			OriginalURL:   episode.URL,
			DownloadURL:   episode.Format.Video.URI,
			Resolution:    episode.Format.Video.Resolution,
			FPS:           episode.Format.Video.FrameRate,
			Audio:         episode.Format.AudioLocale,
			Subtitle:      subtitle,
		}

		if verboseFlag {
			fmtOptionsBytes, err := json.Marshal(info)
			if err != nil {
				fmtOptionsBytes = make([]byte, 0)
			}
			out.Debugf("Information (json): %s\n", string(fmtOptionsBytes))
		}

		filename := outputFlag

		fields := reflect.TypeOf(info)
		values := reflect.ValueOf(info)
		for i := 0; i < fields.NumField(); i++ {
			field := fields.Field(i)
			value := values.Field(i)

			var valueAsString string
			switch value.Kind() {
			case reflect.String:
				valueAsString = value.String()
			case reflect.Float64:
				valueAsString = strconv.Itoa(int(value.Float()))
			}

			filename = strings.ReplaceAll(filename, "{"+field.Tag.Get("json")+"}", valueAsString)
		}

		invalidChars := invalidLinuxChars
		if runtime.GOOS == "windows" {
			invalidChars = invalidWindowsChars
		}

		// replaces all the invalid characters
		for _, char := range invalidChars {
			filename = strings.ReplaceAll(filename, char, "")
		}

		out.Empty()
		if downloadFormat(episode.Format, filename, info) {
			success++
		}
	}

	out.Infof("Downloaded %d out of %d videos\n", success, len(allEpisodes))
}

func parseURLs(urls []string) (allEpisodes []episodeInformation, total, successes int) {
	videoDupes := map[string]utils.VideoStructure{}

	for i, url := range urls {
		out.StartProgressf("Parsing url %d", i+1)

		var localTotal, localSuccesses int

		var seriesName string
		var ok bool
		if seriesName, _, ok = crunchyroll.MatchEpisode(url); !ok {
			seriesName, _ = crunchyroll.MatchVideo(url)
		}

		if seriesName != "" {
			dupe, ok := videoDupes[seriesName]
			if !ok {
				video, err := crunchy.FindVideo(fmt.Sprintf("https://www.crunchyroll.com/%s", seriesName))
				if err != nil {
					continue
				}
				switch video.(type) {
				case *crunchyroll.Series:
					seasons, err := video.(*crunchyroll.Series).Seasons()
					if err != nil {
						out.EndProgressf(false, "Failed to get seasons for url %s: %s\n", url, err)
						continue
					}
					dupe = utils.NewSeasonStructure(seasons).EpisodeStructure
					if err := dupe.(*utils.EpisodeStructure).InitAll(); err != nil {
						out.EndProgressf(false, "Failed to initialize series for url %s\n", url)
						continue
					}
				case *crunchyroll.Movie:
					movieListings, err := video.(*crunchyroll.Movie).MovieListing()
					if err != nil {
						out.EndProgressf(false, "Failed to get movie listing for url %s\n", url)
						continue
					}
					dupe = utils.NewMovieListingStructure(movieListings)
					if err := dupe.(*utils.MovieListingStructure).InitAll(); err != nil {
						out.EndProgressf(false, "Failed to initialize movie for url %s\n", url)
						continue
					}
				}
				videoDupes[seriesName] = dupe
			}

			if _, ok := crunchyroll.MatchVideo(url); ok {
				out.Debugf("Parsed url %d as video\n", i+1)
				var parsed []episodeInformation
				parsed, localTotal, localSuccesses = parseVideo(dupe, url)
				allEpisodes = append(allEpisodes, parsed...)
			} else if _, _, ok = crunchyroll.MatchEpisode(url); ok {
				out.Debugf("Parsed url %d as episode\n", i+1)
				if episode := parseEpisodes(dupe.(*utils.EpisodeStructure), url); (episodeInformation{}) != episode {
					allEpisodes = append(allEpisodes, episode)
					localSuccesses++
				} else {
					out.EndProgressf(false, "Could not parse url %d, skipping\n", i+1)
				}
				localTotal++
			} else {
				out.EndProgressf(false, "Could not parse url %d, skipping\n", i+1)
				continue
			}
			out.EndProgressf(true, "Parsed url %d with %d successes and %d fails\n", i+1, localSuccesses, localTotal-localSuccesses)
		} else {
			out.EndProgressf(false, "URL %d seems to be invalid\n", i+1)
		}
		total += localTotal
		successes += localSuccesses
	}
	return
}

func parseVideo(videoStructure utils.VideoStructure, url string) (episodeInformations []episodeInformation, total, successes int) {
	var orderedFormats [][]*crunchyroll.Format

	switch videoStructure.(type) {
	case *utils.EpisodeStructure:
		orderedFormats, _ = videoStructure.(*utils.EpisodeStructure).OrderFormatsByEpisodeNumber()
	case *utils.MovieListingStructure:
		unorderedFormats, _ := videoStructure.(*utils.MovieListingStructure).Formats()
		orderedFormats = append(orderedFormats, unorderedFormats)
	}

	out.Debugf("Found %d different episodes\n", len(orderedFormats))

	for _, formats := range orderedFormats {
		if formats == nil {
			continue
		}
		total++

		var title string
		switch videoStructure.(type) {
		case *utils.EpisodeStructure:
			episode, _ := videoStructure.(*utils.EpisodeStructure).GetEpisodeByFormat(formats[0])
			title = episode.Title
		case *utils.MovieListingStructure:
			movieListing, _ := videoStructure.(*utils.MovieListingStructure).GetMovieListingByFormat(formats[0])
			title = movieListing.Title
		}

		if format := findFormat(formats, title); format != nil {
			info := episodeInformation{Format: format, URL: url}
			switch videoStructure.(type) {
			case *utils.EpisodeStructure:
				episode, _ := videoStructure.(*utils.EpisodeStructure).GetEpisodeByFormat(format)
				info.Title = episode.Title
				info.SeriesTitle = episode.SeriesTitle
				info.SeasonNum = episode.SeasonNumber
				info.EpisodeNum = episode.EpisodeNumber
			case *utils.MovieListingStructure:
				movieListing, _ := videoStructure.(*utils.MovieListingStructure).GetMovieListingByFormat(format)
				info.Title = movieListing.Title
				info.SeriesTitle = movieListing.Title
				info.SeasonNum, info.EpisodeNum = 1, 1
			}

			episodeInformations = append(episodeInformations, info)
			out.Debugf("Successful parsed %s\n", title)
		}
		successes++
	}

	return
}

func parseEpisodes(episodeStructure *utils.EpisodeStructure, url string) episodeInformation {
	episode, _ := episodeStructure.GetEpisodeByURL(url)
	ordered, _ := episodeStructure.OrderFormatsByEpisodeNumber()

	formats := ordered[episode.EpisodeNumber]
	out.Debugf("Found %d formats\n", len(formats))
	if format := findFormat(formats, episode.Title); format != nil {
		episode, _ = episodeStructure.GetEpisodeByFormat(format)
		out.Debugf("Found matching episode %s\n", episode.Title)
		return episodeInformation{
			Format:      format,
			Title:       episode.Title,
			URL:         url,
			SeriesTitle: episode.SeriesTitle,
			SeasonNum:   episode.SeasonNumber,
			EpisodeNum:  episode.EpisodeNumber,
		}
	}
	return episodeInformation{}
}

func findFormat(formats []*crunchyroll.Format, name string) (format *crunchyroll.Format) {
	formatStructure := utils.NewFormatStructure(formats)
	var audioLocale, subtitleLocale crunchyroll.LOCALE

	if audioFlag != "" {
		audioLocale = localeToLOCALE(audioFlag)
	} else {
		audioLocale = systemLocale()
	}
	if subtitleFlag != "" {
		subtitleLocale = localeToLOCALE(subtitleFlag)
	}

	formats, _ = formatStructure.FilterFormatsByLocales(audioLocale, subtitleLocale, !noHardsubFlag)
	if formats == nil {
		if audioFlag == "" {
			out.Errf("Failed to find episode with '%s' audio and '%s' subtitles, tying with %s audio\n", audioLocale, subtitleLocale, strings.ToLower(utils.LocaleLanguage(crunchyroll.JP)))
			audioLocale = crunchyroll.JP
			formats, _ = formatStructure.FilterFormatsByLocales(audioLocale, subtitleLocale, !noHardsubFlag)
		}
		if formats == nil && subtitleFlag == "" {
			out.Errf("Failed to find episode with '%s' audio and '%s' subtitles, tying with %s subtitle\n", audioLocale, subtitleLocale, strings.ToLower(utils.LocaleLanguage(systemLocale())))
			subtitleLocale = systemLocale()
			formats, _ = formatStructure.FilterFormatsByLocales(audioLocale, subtitleLocale, !noHardsubFlag)
		}
		if formats == nil {
			out.Errf("Could not find matching video with '%s' audio and '%s' subtitles for %s. Try to change the '--audio' and / or '--subtitle' flag\n", audioLocale, subtitleLocale, name)
			return nil
		}
	}
	if resolutionFlag == "best" || resolutionFlag == "" {
		sort.Sort(sort.Reverse(utils.FormatsByResolution(formats)))
		format = formats[0]
	} else if resolutionFlag == "worst" {
		sort.Sort(utils.FormatsByResolution(formats))
		format = formats[0]
	} else if strings.HasSuffix(resolutionFlag, "p") {
		for _, f := range formats {
			if strings.Split(f.Video.Resolution, "x")[1] == strings.TrimSuffix(resolutionFlag, "p") {
				format = f
				break
			}
		}
	} else if strings.Contains(resolutionFlag, "x") {
		for _, f := range formats {
			if f.Video.Resolution == resolutionFlag {
				format = f
				break
			}
		}
	}
	if format == nil {
		out.Errf("Failed to get video with resolution '%s'\n", resolutionFlag)
	}

	subtitleFlag = string(subtitleLocale)
	return
}

func downloadFormat(format *crunchyroll.Format, outFile string, info information) bool {
	oldOutFile := outFile
	outFile, changed := freeFileName(outFile)
	ext := path.Ext(outFile)
	out.Debugf("Download filename: %s\n", outFile)
	if changed {
		out.Errf("The file %s already exist, renaming the download file to %s\n", oldOutFile, outFile)
	}
	if ext != ".ts" {
		if !hasFFmpeg() {
			out.Fatalf("The file ending for the output file (%s) is not `.ts`. "+
				"Install ffmpeg (https://ffmpeg.org/download.html) use other media file endings (e.g. `.mp4`)\n", outFile)
		}
		out.Debugf("File will be converted via ffmpeg")
	}
	var subtitleFilename string
	if noHardsubFlag {
		subtitle, ok := utils.SubtitleByLocale(format, info.Subtitle)
		if !ok {
			out.Errf("Failed to get %s subtitles\n", info.Subtitle)
			return false
		}
		subtitleFilename, _ = freeFileName(fmt.Sprintf("%s.%s", strings.TrimSuffix(outFile, ext), subtitle.Format))
		out.Debugf("Subtitles will be saved as '%s'\n", subtitleFilename)
	}

	out.Infof("Downloading '%s' (%s) as '%s'\n", info.Title, info.OriginalURL, outFile)
	out.Infof("Series: %s\n", info.SeriesName)
	out.Infof("Season & Episode: S%02dE%02d", info.SeasonNumber, info.EpisodeNumber)
	out.Infof("Audio: %s\n", info.Audio)
	out.Infof("Subtitle: %s\n", info.Subtitle)
	out.Infof("Hardsub: %v\n", format.Hardsub != "")
	out.Infof("Resolution: %s\n", info.Resolution)
	out.Infof("FPS: %.2f\n", info.FPS)

	var err error
	if ext == ".ts" {
		file, err := os.Create(outFile)
		defer file.Close()
		if err != nil {
			out.Errf("Could not create file '%s' to download episode '%s' (%s): %s, skipping\n", outFile, info.Title, info.OriginalURL, err)
			return false
		}

		err = format.Download(file, downloadProgress)
		// newline to avoid weird output
		fmt.Println()
	} else {
		tempDir, err := os.MkdirTemp("", "crunchy_")
		if err != nil {
			out.Errln("Failed to create temp download dir. Skipping")
			return false
		}

		var segmentCount int
		err = format.DownloadSegments(tempDir, 4, func(segment *m3u8.MediaSegment, current, total int, file *os.File, err error) error {
			segmentCount++
			return downloadProgress(segment, current, total, file, err)
		})
		// newline to avoid weird output
		fmt.Println()

		f, _ := os.CreateTemp("", "*.txt")
		for i := 0; i < segmentCount; i++ {
			fmt.Fprintf(f, "file '%s.ts'\n", filepath.Join(tempDir, strconv.Itoa(i)))
		}
		defer os.Remove(f.Name())
		f.Close()

		cmd := exec.Command("ffmpeg",
			"-f", "concat",
			"-safe", "0",
			"-i", f.Name(),
			"-c", "copy",
			outFile)
		err = cmd.Run()
	}
	os.RemoveAll(cleanupPath)
	cleanupPath = ""

	if err != nil {
		out.Errln("Failed to download video, skipping")
	} else {
		if info.Subtitle == "" {
			out.Infof("Downloaded '%s' as '%s' with %s audio locale\n", info.Title, outFile, strings.ToLower(utils.LocaleLanguage(info.Audio)))
		} else {
			out.Infof("Downloaded '%s' as '%s' with %s audio locale and %s subtitle locale\n", info.Title, outFile, strings.ToLower(utils.LocaleLanguage(info.Audio)), strings.ToLower(utils.LocaleLanguage(info.Subtitle)))
			if subtitleFilename != "" {
				file, err := os.Create(subtitleFilename)
				if err != nil {
					out.Errf("Failed to download subtitles: %s\n", err)
					return false
				} else {
					subtitle, ok := utils.SubtitleByLocale(format, info.Subtitle)
					if !ok {
						out.Errf("Failed to get %s subtitles\n", info.Subtitle)
						return false
					}
					if err := subtitle.Download(file); err != nil {
						out.Errf("Failed to download subtitles: %s\n", err)
						return false
					}
					out.Infof("Downloaded '%s' subtitles to '%s'\n", info.Subtitle, subtitleFilename)
				}
			}
		}
	}

	return true
}

func downloadProgress(segment *m3u8.MediaSegment, current, total int, file *os.File, err error) error {
	if cleanupPath == "" {
		cleanupPath = path.Dir(file.Name())
	}

	if !quietFlag {
		percentage := float32(current) / float32(total) * 100
		if alternativeProgressFlag {
			out.Infof("Downloading %d/%d (%.2f%%) » %s", current, total, percentage, segment.URI)
		} else {
			progressWidth := float32(terminalWidth() - (14 + len(out.InfoLog.Prefix())) - (len(fmt.Sprint(total)))*2)

			repeatCount := int(percentage / (float32(100) / progressWidth))
			// it can be lower than zero when the terminal is very tiny
			if repeatCount < 0 {
				repeatCount = 0
			}

			// alternative:
			// 		progressPercentage := strings.Repeat("█", repeatCount)
			progressPercentage := (strings.Repeat("=", repeatCount) + ">")[1:]

			fmt.Printf("\r%s[%-"+fmt.Sprint(progressWidth)+"s]%4d%% %8d/%d", out.InfoLog.Prefix(), progressPercentage, int(percentage), current, total)
		}
	}
	return nil
}
