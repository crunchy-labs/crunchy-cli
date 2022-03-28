package cmd

import (
	"context"
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/ByteDream/crunchyroll-go/utils"
	"github.com/grafov/m3u8"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var (
	downloadAudioFlag    string
	downloadSubtitleFlag string

	downloadDirectoryFlag string
	downloadOutputFlag    string

	downloadResolutionFlag string

	downloadGoroutinesFlag int
)

var getCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a video",
	Args:  cobra.MinimumNArgs(1),

	PreRunE: func(cmd *cobra.Command, args []string) error {
		out.Debug("Validating arguments")

		if filepath.Ext(downloadOutputFlag) != ".ts" {
			if !hasFFmpeg() {
				return fmt.Errorf("the file ending for the output file (%s) is not `.ts`. "+
					"Install ffmpeg (https://ffmpeg.org/download.html) to use other media file endings (e.g. `.mp4`)", downloadOutputFlag)
			} else {
				out.Debug("Custom file ending '%s' (ffmpeg is installed)", filepath.Ext(downloadOutputFlag))
			}
		}

		if !utils.ValidateLocale(crunchyroll.LOCALE(downloadAudioFlag)) {
			return fmt.Errorf("%s is not a valid audio locale. Choose from: %s", downloadAudioFlag, strings.Join(allLocalesAsStrings(), ", "))
		} else if downloadSubtitleFlag != "" && !utils.ValidateLocale(crunchyroll.LOCALE(downloadSubtitleFlag)) {
			return fmt.Errorf("%s is not a valid subtitle locale. Choose from: %s", downloadSubtitleFlag, strings.Join(allLocalesAsStrings(), ", "))
		}
		out.Debug("Locales: audio: %s / subtitle: %s", downloadAudioFlag, downloadSubtitleFlag)

		switch downloadResolutionFlag {
		case "1080p", "720p", "480p", "360p", "240p":
			intRes, _ := strconv.ParseFloat(strings.TrimSuffix(downloadResolutionFlag, "p"), 84)
			downloadResolutionFlag = fmt.Sprintf("%dx%s", int(intRes*(16/9)), strings.TrimSuffix(downloadResolutionFlag, "p"))
		case "1920x1080", "1280x720", "640x480", "480x360", "428x240", "best", "worst":
		default:
			return fmt.Errorf("'%s' is not a valid resolution", downloadResolutionFlag)
		}
		out.Debug("Using resolution '%s'", downloadResolutionFlag)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		loadCrunchy()

		return download(args)
	},
}

func init() {
	getCmd.Flags().StringVarP(&downloadAudioFlag, "audio",
		"a",
		string(systemLocale(false)),
		"The locale of the audio. Available locales: "+strings.Join(allLocalesAsStrings(), ", "))
	getCmd.Flags().StringVarP(&downloadSubtitleFlag,
		"subtitle",
		"s",
		"",
		"The locale of the subtitle. Available locales: "+strings.Join(allLocalesAsStrings(), ", "))

	cwd, _ := os.Getwd()
	getCmd.Flags().StringVarP(&downloadDirectoryFlag,
		"directory",
		"d",
		cwd,
		"The directory to download the file(s) into")
	getCmd.Flags().StringVarP(&downloadOutputFlag,
		"output",
		"o",
		"{title}.ts",
		"Name of the output file. "+
			"If you use the following things in the name, the will get replaced:\n"+
			"\t{title} » Title of the video\n"+
			"\t{series_name} » Name of the series\n"+
			"\t{season_name} » Name of the season\n"+
			"\t{season_number} » Number of the season\n"+
			"\t{episode_number} » Number of the episode\n"+
			"\t{resolution} » Resolution of the video\n"+
			"\t{fps} » Frame Rate of the video\n"+
			"\t{audio} » Audio locale of the video\n"+
			"\t{subtitle} » Subtitle locale of the video")

	getCmd.Flags().StringVarP(&downloadResolutionFlag,
		"resolution",
		"r",
		"best",
		"The video resolution. Can either be specified via the pixels, the abbreviation for pixels, or 'common-use' words\n"+
			"\tAvailable pixels: 1920x1080, 1280x720, 640x480, 480x360, 428x240\n"+
			"\tAvailable abbreviations: 1080p, 720p, 480p, 360p, 240p\n"+
			"\tAvailable common-use words: best (best available resolution), worst (worst available resolution)")

	getCmd.Flags().IntVarP(&downloadGoroutinesFlag,
		"goroutines",
		"g",
		runtime.NumCPU(),
		"Sets how many parallel segment downloads should be used")

	rootCmd.AddCommand(getCmd)
}

func download(urls []string) error {
	for i, url := range urls {
		out.SetProgress("Parsing url %d", i+1)
		episodes, err := downloadExtractEpisodes(url)
		if err != nil {
			out.StopProgress("Failed to parse url %d", i+1)
			return err
		}
		out.StopProgress("Parsed url %d", i+1)

		for _, season := range episodes {
			out.Info("%s Season %d", season[0].SeriesName, season[0].SeasonNumber)

			for j, info := range season {
				out.Info("\t%d. %s » %spx, %.2f FPS (S%02dE%02d)",
					j+1,
					info.Title,
					info.Resolution,
					info.FPS,
					info.SeasonNumber,
					info.EpisodeNumber)
			}
		}
		out.Empty()

		for _, season := range episodes {
			for _, info := range season {
				dir := info.Format(downloadDirectoryFlag)
				if _, err = os.Stat(dir); os.IsNotExist(err) {
					if err = os.MkdirAll(dir, 0777); err != nil {
						return fmt.Errorf("error while creating directory: %v", err)
					}
				}
				file, err := os.Create(generateFilename(info.Format(downloadOutputFlag), dir))
				if err != nil {
					return fmt.Errorf("failed to create output file: %v", err)
				}

				if err = downloadInfo(info, file); err != nil {
					file.Close()
					os.Remove(file.Name())
					return err
				}
				file.Close()
			}
		}
	}
	return nil
}

func downloadInfo(info formatInformation, file *os.File) error {
	out.Debug("Entering season %d, episode %d", info.SeasonNumber, info.EpisodeNumber)

	if err := info.format.InitVideo(); err != nil {
		return fmt.Errorf("error while initializing the video: %v", err)
	}

	dp := &downloadProgress{
		Prefix:  out.InfoLog.Prefix(),
		Message: "Downloading video",
		// number of segments a video has +2 is for merging and the success message
		Total: int(info.format.Video.Chunklist.Count()) + 2,
		Dev:   out.IsDev(),
		Quiet: out.IsQuiet(),
	}
	if out.IsDev() {
		dp.Prefix = out.DebugLog.Prefix()
	}
	defer func() {
		if dp.Total != dp.Current {
			fmt.Println()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	downloader := crunchyroll.NewDownloader(ctx, file, downloadGoroutinesFlag, func(segment *m3u8.MediaSegment, current, total int, file *os.File) error {
		// check if the context was cancelled.
		// must be done in to not print any progress messages if ctrl+c was pressed
		if ctx.Err() != nil {
			return nil
		}

		if out.IsDev() {
			dp.UpdateMessage(fmt.Sprintf("Downloading %d/%d (%.2f%%) » %s", current, total, float32(current)/float32(total)*100, segment.URI), false)
		} else {
			dp.Update()
		}

		if current == total {
			dp.UpdateMessage("Merging segments", false)
		}
		return nil
	})
	if hasFFmpeg() {
		downloader.FFmpegOpts = make([]string, 0)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		select {
		case <-sig:
			signal.Stop(sig)
			out.Exit("Exiting... (may take a few seconds)")
			out.Exit("To force exit press ctrl+c (again)")
			cancel()
			// os.Exit(1) is not called because an immediate exit after the cancel function does not let
			// the download process enough time to stop gratefully. A result of this is that the temporary
			// directory where the segments are downloaded to will not be deleted
		case <-ctx.Done():
			// this is just here to end the goroutine and prevent it from running forever without a reason
		}
	}()
	out.Debug("Set up signal catcher")

	out.Info("Downloading episode `%s` to `%s`", info.Title, filepath.Base(file.Name()))
	out.Info("\tEpisode: S%02dE%02d", info.SeasonNumber, info.EpisodeNumber)
	out.Info("\tAudio: %s", info.Audio)
	out.Info("\tSubtitle: %s", info.Subtitle)
	out.Info("\tResolution: %spx", info.Resolution)
	out.Info("\tFPS: %.2f", info.FPS)
	if err := info.format.Download(downloader); err != nil {
		return fmt.Errorf("error while downloading: %v", err)
	}

	dp.UpdateMessage("Download finished", false)

	signal.Stop(sig)
	out.Debug("Stopped signal catcher")

	out.Empty()
	out.Empty()

	return nil
}

func downloadExtractEpisodes(url string) ([][]formatInformation, error) {
	episodes, err := extractEpisodes(url, crunchyroll.JP, crunchyroll.LOCALE(downloadAudioFlag))
	if err != nil {
		return nil, err
	}
	japanese := episodes[0]
	custom := episodes[1]

	sort.Sort(utils.EpisodesByNumber(japanese))
	sort.Sort(utils.EpisodesByNumber(custom))

	var errMessages []string

	var final []*crunchyroll.Episode
	if len(japanese) == 0 || len(japanese) == len(custom) {
		final = custom
	} else {
		for _, jp := range japanese {
			before := len(final)
			for _, episode := range custom {
				if jp.SeasonNumber == episode.SeasonNumber && jp.EpisodeNumber == episode.EpisodeNumber {
					final = append(final, episode)
				}
			}
			if before == len(final) {
				errMessages = append(errMessages, fmt.Sprintf("%s has no %s audio, using %s as fallback", jp.Title, crunchyroll.LOCALE(downloadAudioFlag), crunchyroll.JP))
				final = append(final, jp)
			}
		}
	}

	if len(errMessages) > 10 {
		for _, msg := range errMessages[:10] {
			out.SetProgress(msg)
		}
		out.SetProgress("... and %d more", len(errMessages)-10)
	} else {
		for _, msg := range errMessages {
			out.SetProgress(msg)
		}
	}

	var infoFormat [][]formatInformation
	for _, season := range utils.SortEpisodesBySeason(final) {
		tmpFormatInformation := make([]formatInformation, 0)
		for _, episode := range season {
			format, err := episode.GetFormat(downloadResolutionFlag, crunchyroll.LOCALE(downloadSubtitleFlag), true)
			if err != nil {
				return nil, fmt.Errorf("error while receiving format for %s: %v", episode.Title, err)
			}
			tmpFormatInformation = append(tmpFormatInformation, formatInformation{
				format: format,

				Title:         episode.Title,
				SeriesName:    episode.SeriesTitle,
				SeasonName:    episode.SeasonTitle,
				SeasonNumber:  episode.SeasonNumber,
				EpisodeNumber: episode.EpisodeNumber,
				Resolution:    format.Video.Resolution,
				FPS:           format.Video.FrameRate,
				Audio:         format.AudioLocale,
			})
		}
		infoFormat = append(infoFormat, tmpFormatInformation)
	}
	return infoFormat, nil
}
