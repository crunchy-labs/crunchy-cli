package cmd

import (
	"bytes"
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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/template"
)

// sigusr1 is actually syscall.SIGUSR1, but because has no signal (or very less) it has to be defined manually
var sigusr1 = syscall.Signal(0xa)

var (
	audioFlag     string
	subtitleFlag  string
	noHardsubFlag bool

	directoryFlag string
	outputFlag    string

	resolutionFlag string

	alternativeProgressFlag bool
)

var cleanup [2]string

var getCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a video",
	Args:  cobra.MinimumNArgs(1),

	Run: func(cmd *cobra.Command, args []string) {
		loadCrunchy()
		download(args)
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&audioFlag, "audio", "", "The locale of the audio. Available locales: "+strings.Join(allLocalesAsStrings(), ", "))
	getCmd.Flags().StringVar(&subtitleFlag, "subtitle", "", "The locale of the subtitle. Available locales: "+strings.Join(allLocalesAsStrings(), ", "))
	getCmd.Flags().BoolVar(&noHardsubFlag, "no-hardsub", false, "Same as `--sub`, but the subtitles are not stored in the video itself, but in a separate file")

	cwd, _ := os.Getwd()
	getCmd.Flags().StringVarP(&directoryFlag, "directory", "d", cwd, "The directory to download the file to")
	getCmd.Flags().StringVarP(&outputFlag, "output", "o", "{{.Title}}.ts", "Name of the output file\n"+
		"If you use the following things in the name, the will get replaced"+
		"\t{{.Title}} » Title of the video\n"+
		"\t{{.Resolution}} » Resolution of the video\n"+
		"\t{{.FPS}} » Frame Rate of the video\n"+
		"\t{{.Audio}} » Audio locale of the video\n"+
		"\t{{.Subtitle}} » Subtitle locale of the video\n")

	getCmd.Flags().StringVarP(&resolutionFlag, "resolution", "r", "best", "res")

	getCmd.Flags().BoolVar(&alternativeProgressFlag, "alternative-progress", false, "Shows an alternative, not so user-friendly progress instead of the progress bar")
}

type information struct {
	Title       string             `json:"title"`
	OriginalURL string             `json:"original_url"`
	DownloadURL string             `json:"download_url"`
	Resolution  string             `json:"resolution"`
	FPS         float64            `json:"fps"`
	Audio       crunchyroll.LOCALE `json:"audio"`
	Subtitle    crunchyroll.LOCALE `json:"subtitle"`
	Hardsub     bool               `json:"hardsub"`
}

func download(urls []string) {
	if path.Ext(outputFlag) != ".ts" && !hasFFmpeg() {
		out.Fatalf("The file ending for the output file (%s) is not `.ts`. "+
			"Install ffmpeg (https://ffmpeg.org/download.html) use other media file endings (e.g. `.mp4`)\n", outputFlag)
	}

	var allFormats []*crunchyroll.Format
	var allTitles []string
	var allURLs []string

	for i, url := range urls {
		var failed bool

		out.StartProgressf("Parsing url %d", i+1)
		if video, err1 := crunchy.FindVideo(url); err1 == nil {
			out.Debugf("Pre-parsed url %d as video\n", i+1)
			if formats, titles := parseVideo(video, url); formats != nil {
				allFormats = append(allFormats, formats...)
				allTitles = append(allTitles, titles...)
				for range formats {
					allURLs = append(allURLs, url)
				}
			} else {
				failed = true
			}
		} else if episodes, err2 := crunchy.FindEpisode(url); err2 == nil {
			out.Debugf("Parsed url %d as episode\n", i+1)
			out.Debugf("Found %d episode types\n", len(episodes))
			if format, title := parseEpisodes(episodes, url); format != nil {
				allFormats = append(allFormats, format)
				allTitles = append(allTitles, title)
				allURLs = append(allURLs, url)
			} else {
				failed = true
			}
		} else {
			out.EndProgressf(false, "Could not parse url %d, skipping\n", i+1)
			out.Debugf("Parse error 1: %s\n", err1)
			out.Debugf("Parse error 2: %s\n", err2)
			continue
		}

		if !failed {
			out.EndProgressf(true, "Parsed url %d successful\n", i+1)
		} else {
			out.EndProgressf(false, "Failed to parse url %d (the url is valid but some kind of error which is surely shown caused the failure)", i+1)
		}
	}
	out.Debugf("%d of %d urls could be parsed\n", len(allURLs), len(urls))

	out.Empty()
	if len(allFormats) == 0 {
		out.Fatalf("Nothing to download, aborting\n")
	}
	out.Infof("Downloads:")
	for i, format := range allFormats {
		video := format.Video
		out.Infof("\t%d. %s » %spx, %.2f FPS, %s audio\n", i+1, allTitles[i], video.Resolution, video.FrameRate, utils.LocaleLanguage(format.AudioLocale))
	}
	var tmpl *template.Template
	var err error
	tmpl, err = template.New("").Parse(outputFlag)
	if err == nil {
		var buff bytes.Buffer
		if err := tmpl.Execute(&buff, allFormats[0].Video); err == nil {
			if buff.String() == outputFlag {
				tmpl = nil
			}
		}
	}

	if fileInfo, stat := os.Stat(directoryFlag); err == nil {
		if !fileInfo.IsDir() {
			out.Fatalf("%s (given from the `-d`/`--directory` flag) is not a directory\n", directoryFlag)
		}
	} else if os.IsNotExist(stat) {
		if err := os.MkdirAll(directoryFlag, 0777); err != nil {
			out.Fatalf("Failed to create directory which was given from the `-d`/`--directory` flag: %s\n", err)
		}
	} else {
		out.Fatalf("Failed to get information for via `-d`/`--directory` flag the given file / directory: %s", err)
	}

	var success int
	for i, format := range allFormats {
		var subtitle crunchyroll.LOCALE
		if subtitleFlag != "" {
			subtitle = localeToLOCALE(subtitleFlag)
		}
		info := information{
			Title:       allTitles[i],
			OriginalURL: allURLs[i],
			DownloadURL: format.Video.URI,
			Resolution:  format.Video.Resolution,
			FPS:         format.Video.FrameRate,
			Audio:       format.AudioLocale,
			Subtitle:    subtitle,
		}

		if verboseFlag {
			fmtOptionsBytes, err := json.Marshal(info)
			if err != nil {
				fmtOptionsBytes = make([]byte, 0)
			}
			out.Debugf("Information (json): %s", string(fmtOptionsBytes))
		}

		var baseFilename string
		if tmpl != nil {
			var buff bytes.Buffer
			if err := tmpl.Execute(&buff, info); err == nil {
				baseFilename = buff.String()
			} else {
				out.Fatalf("Could not convert filename (%s), aborting\n", err)
			}
		} else {
			baseFilename = outputFlag
		}

		out.Empty()
		if downloadFormat(format, directoryFlag, baseFilename, info) {
			success++
		}
	}

	out.Empty()
	out.Infof("Downloaded %d out of %d videos successful\n", success, len(allFormats))
}

func parseVideo(video crunchyroll.Video, url string) (parsedFormats []*crunchyroll.Format, titles []string) {
	var rootTitle string
	var orderedFormats [][]*crunchyroll.Format
	var videoStructure utils.VideoStructure

	switch video.(type) {
	case *crunchyroll.Series:
		out.Debugf("Parsed url as series\n")
		series := video.(*crunchyroll.Series)
		seasons, err := series.Seasons()
		if err != nil {
			out.Errf("Could not get any season of %s (%s): %s. Aborting\n", series.Title, url, err.Error())
			return
		}
		out.Debugf("Found %d seasons\n", len(seasons))
		seasonsStructure := utils.NewSeasonStructure(seasons)
		if err := seasonsStructure.InitAll(); err != nil {
			out.Errf("Failed to initialize %s (%s): %s. Aborting\n", series.Title, url, err.Error())
			return
		}
		out.Debugf("Initialized %s\n", series.Title)

		rootTitle = series.Title
		orderedFormats, _ = seasonsStructure.OrderFormatsByEpisodeNumber()
		videoStructure = seasonsStructure.EpisodeStructure
	case *crunchyroll.Movie:
		out.Debugf("Parsed url as movie\n")
		movie := video.(*crunchyroll.Movie)
		movieListings, err := movie.MovieListing()
		if err != nil {
			out.Errf("Failed to get movie of %s (%s)\n", movie.Title, url)
			return
		}
		out.Debugf("Parsed %d movie listenings\n", len(movieListings))
		movieListingStructure := utils.NewMovieListingStructure(movieListings)
		if err := movieListingStructure.InitAll(); err != nil {
			out.Errf("Failed to initialize %s (%s): %s. Aborting\n", movie.Title, url, err.Error())
			return
		}

		rootTitle = movie.Title
		unorderedFormats, _ := movieListingStructure.Formats()
		orderedFormats = append(orderedFormats, unorderedFormats)
		videoStructure = movieListingStructure
	}

	// out.Debugf("Found %d formats\n", len(unorderedFormats))
	out.Debugf("Found %d different episodes\n", len(orderedFormats))

	for j, formats := range orderedFormats {
		if format := findFormat(formats); format != nil {
			var title string
			switch videoStructure.(type) {
			case *utils.EpisodeStructure:
				episode, _ := videoStructure.(*utils.EpisodeStructure).GetEpisodeByFormat(format)
				title = episode.Title
			case *utils.MovieListingStructure:
				movieListing, _ := videoStructure.(*utils.MovieListingStructure).GetMovieListingByFormat(format)
				title = movieListing.Title
			}

			parsedFormats = append(parsedFormats, format)
			titles = append(titles, title)
			out.Debugf("Successful parsed format %d for %s\n", j+1, rootTitle)
		}
	}

	return
}

func parseEpisodes(episodes []*crunchyroll.Episode, url string) (*crunchyroll.Format, string) {
	episodeStructure := utils.NewEpisodeStructure(episodes)
	if err := episodeStructure.InitAll(); err != nil {
		out.EndProgressf(false, "Failed to initialize %s (%s): %s, skipping\n", episodes[0].Title, url, err)
		return nil, ""
	}

	formats, _ := episodeStructure.Formats()
	out.Debugf("Found %d formats\n", len(formats))
	if format := findFormat(formats); format != nil {
		episode, _ := episodeStructure.GetEpisodeByFormat(format)
		return format, episode.Title
	}
	return nil, ""
}

func findFormat(formats []*crunchyroll.Format) (format *crunchyroll.Format) {
	formatStructure := utils.NewFormatStructure(formats)
	var audioLocale, subtitleLocale crunchyroll.LOCALE

	if audioFlag != "" {
		audioLocale = localeToLOCALE(audioFlag)
	} else {
		audioLocale = localeToLOCALE(systemLocale())
	}
	if subtitleFlag != "" {
		subtitleLocale = localeToLOCALE(subtitleFlag)
	}

	if audioFlag == "" {
		var dubOk bool
		availableDub, _, _, _ := formatStructure.AvailableLocales(true)
		for _, dub := range availableDub {
			if dub == audioLocale {
				dubOk = true
				break
			}
		}
		if !dubOk {
			if audioFlag != systemLocale() {
				out.EndProgressf(false, "No stream with audio locale `%s` is available, skipping\n", audioLocale)
				return nil
			}
			out.Errf("No stream with default audio locale `%s` is available, using hardsubbed %s with subtitle locale %s\n", audioLocale, crunchyroll.JP, systemLocale())
			audioLocale = crunchyroll.JP
			if subtitleFlag == "" {
				subtitleLocale = localeToLOCALE(systemLocale())
			}
		}
	}

	var dubOk, subOk bool
	availableDub, availableSub, _, _ := formatStructure.AvailableLocales(true)
	for _, dub := range availableDub {
		if dub == audioLocale {
			dubOk = true
			break
		}
	}
	if !dubOk {
		if audioFlag == "" {
			audioLocale = crunchyroll.JP
			if subtitleFlag == "" {
				subtitleLocale = localeToLOCALE(systemLocale())
				out.Errf("No stream with default audio locale `%s` is available, using hardsubbed %s with subtitle locale %s\n", audioLocale, crunchyroll.JP, subtitleLocale)
			}
		}
		for _, dub := range availableDub {
			if dub == audioLocale {
				dubOk = true
				break
			}
		}
	}
	if subtitleLocale != "" {
		for _, sub := range availableSub {
			if sub == subtitleLocale {
				subOk = true
				break
			}
		}
	} else {
		subOk = true
	}

	if !dubOk {
		out.Errf("Could not find any video with `%s` audio locale\n", audioLocale)
	}
	if !subOk {
		out.Errf("Could not find any video with `%s` subtitle locale\n", subtitleLocale)
	}
	if !dubOk || !subOk {
		return nil
	}

	formats, err := formatStructure.FilterFormatsByLocales(audioLocale, subtitleLocale, !noHardsubFlag)
	if err != nil {
		out.Errln("Failed to get matching format. Try to change the `--audio` or `--subtitle` flag")
		return
	}

	if resolutionFlag == "best" || resolutionFlag == "" {
		sort.Sort(sort.Reverse(utils.FormatsByResolution(formats)))
		format = formats[0]
	} else if resolutionFlag == "worst" {
		sort.Sort(utils.FormatsByResolution(formats))
		format = formats[0]
	} else {
		for _, f := range formats {
			if f.Video.Resolution == resolutionFlag {
				format = f
				break
			}
		}
	}
	subtitleFlag = string(subtitleLocale)
	return
}

func downloadFormat(format *crunchyroll.Format, dir, fname string, info information) bool {
	filename := freeFileName(filepath.Join(dir, fname))
	ext := path.Ext(filename)
	out.Debugf("Download filename: %s\n", filename)
	if filename != filepath.Join(dir, fname) {
		out.Errf("The file %s already exist, renaming the download file to %s\n", filepath.Join(dir, fname), filename)
	}
	if ext != ".ts" {
		if !hasFFmpeg() {
			out.Fatalf("The file ending for the output file (%s) is not `.ts`. "+
				"Install ffmpeg (https://ffmpeg.org/download.html) use other media file endings (e.g. `.mp4`)\n", filename)
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
		subtitleFilename = freeFileName(filepath.Join(dir, fmt.Sprintf("%s.%s", strings.TrimRight(path.Base(filename), ext), subtitle.Format)))
		out.Debugf("Subtitles will be saved as `%s`\n", subtitleFilename)
	}

	out.Infof("Downloading `%s` (%s) as `%s`\n", info.Title, info.OriginalURL, filename)
	out.Infof("Audio: %s\n", info.Audio)
	out.Infof("Subtitle: %s\n", info.Subtitle)
	out.Infof("Hardsub: %v\n", format.Hardsub != "")
	out.Infof("Resolution: %s\n", info.Resolution)
	out.Infof("FPS: %.2f\n", info.FPS)

	var err error
	if ext == ".ts" {
		file, err := os.Create(filename)
		defer file.Close()
		if err != nil {
			out.Errf("Could not create file `%s` to download episode `%s` (%s): %s, skipping\n", filename, info.Title, info.OriginalURL, err)
			return false
		}
		cleanup[0] = filename

		// removes all files in case of an unexpected exit
		sigs := make(chan os.Signal)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, sigusr1)
		go func() {
			sig := <-sigs
			os.RemoveAll(cleanup[1])
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				os.Remove(cleanup[0])
				os.Exit(1)
			}
		}()

		err = format.Download(file, downloadProgress)
		// newline to avoid weird output
		fmt.Println()

		// make the goroutine stop
		sigs <- sigusr1
	} else {
		tempDir, err := os.MkdirTemp("", "crunchy_")
		if err != nil {
			out.Errln("Failed to create temp download dir. Skipping")
			return false
		}
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, sigusr1)
		go func() {
			sig := <-sigs
			os.RemoveAll(tempDir)
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				os.Exit(1)
			}
		}()

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
			filename)
		err = cmd.Run()

		sigs <- sigusr1
	}
	if err != nil {
		out.Errln("Failed to download video, skipping")
	} else {
		if info.Subtitle == "" {
			out.Infof("Downloaded `%s` as `%s` with %s audio locale\n", info.Title, filename, strings.ToLower(utils.LocaleLanguage(info.Audio)))
		} else {
			out.Infof("Downloaded `%s` as `%s` with %s audio locale and %s subtitle locale\n", info.Title, filename, strings.ToLower(utils.LocaleLanguage(info.Audio)), strings.ToLower(utils.LocaleLanguage(info.Subtitle)))
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
					out.Infof("Downloaded `%s` subtitles to `%s`\n", info.Subtitle, subtitleFilename)
				}
			}
		}
	}

	return true
}

func downloadProgress(segment *m3u8.MediaSegment, current, total int, file *os.File, err error) error {
	if cleanup[1] == "" && file != nil {
		cleanup[1] = path.Dir(file.Name())
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

func freeFileName(filename string) string {
	ext := path.Ext(filename)
	base := strings.TrimRight(filename, ext)
	for j := 0; ; j++ {
		if _, stat := os.Stat(filename); stat != nil && !os.IsExist(stat) {
			break
		}
		filename = fmt.Sprintf("%s (%d)%s", base, j, ext)
	}
	return filename
}
