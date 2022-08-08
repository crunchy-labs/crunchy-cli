package download

import (
	"context"
	"fmt"
	"github.com/crunchy-labs/crunchy-cli/cli/commands"
	"github.com/crunchy-labs/crunchy-cli/utils"
	"github.com/crunchy-labs/crunchyroll-go/v3"
	crunchyUtils "github.com/crunchy-labs/crunchyroll-go/v3/utils"
	"github.com/grafov/m3u8"
	"github.com/spf13/cobra"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	downloadSubtitleFlag string

	downloadDirectoryFlag string
	downloadOutputFlag    string
	downloadTempDirFlag   string

	downloadResolutionFlag string

	downloadGoroutinesFlag int
)

var Cmd = &cobra.Command{
	Use:   "download",
	Short: "Download a video",
	Args:  cobra.MinimumNArgs(1),

	PreRunE: func(cmd *cobra.Command, args []string) error {
		utils.Log.Debug("Validating arguments")

		if filepath.Ext(downloadOutputFlag) != ".ts" {
			if !utils.HasFFmpeg() {
				return fmt.Errorf("the file ending for the output file (%s) is not `.ts`. "+
					"Install ffmpeg (https://ffmpeg.org/download.html) to use other media file endings (e.g. `.mp4`)", downloadOutputFlag)
			} else {
				utils.Log.Debug("Custom file ending '%s' (ffmpeg is installed)", filepath.Ext(downloadOutputFlag))
			}
		}

		if downloadSubtitleFlag != "" && !crunchyUtils.ValidateLocale(crunchyroll.LOCALE(downloadSubtitleFlag)) {
			return fmt.Errorf("%s is not a valid subtitle locale. Choose from: %s", downloadSubtitleFlag, strings.Join(utils.LocalesAsStrings(), ", "))
		}
		utils.Log.Debug("Subtitle locale: %s", downloadSubtitleFlag)

		switch downloadResolutionFlag {
		case "1080p", "720p", "480p", "360p":
			intRes, _ := strconv.ParseFloat(strings.TrimSuffix(downloadResolutionFlag, "p"), 84)
			downloadResolutionFlag = fmt.Sprintf("%.0fx%s", math.Ceil(intRes*(float64(16)/float64(9))), strings.TrimSuffix(downloadResolutionFlag, "p"))
		case "240p":
			// 240p would round up to 427x240 if used in the case statement above, so it has to be handled separately
			downloadResolutionFlag = "428x240"
		case "1920x1080", "1280x720", "640x480", "480x360", "428x240", "best", "worst":
		default:
			return fmt.Errorf("'%s' is not a valid resolution", downloadResolutionFlag)
		}
		utils.Log.Debug("Using resolution '%s'", downloadResolutionFlag)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := commands.LoadCrunchy(); err != nil {
			return err
		}

		return download(args)
	},
}

func init() {
	Cmd.Flags().StringVarP(&downloadSubtitleFlag,
		"subtitle",
		"s",
		"",
		"The locale of the subtitle. Available locales: "+strings.Join(utils.LocalesAsStrings(), ", "))

	cwd, _ := os.Getwd()
	Cmd.Flags().StringVarP(&downloadDirectoryFlag,
		"directory",
		"d",
		cwd,
		"The directory to download the file(s) into")
	Cmd.Flags().StringVarP(&downloadOutputFlag,
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
	Cmd.Flags().StringVar(&downloadTempDirFlag,
		"temp",
		os.TempDir(),
		"Directory to store temporary files in")

	Cmd.Flags().StringVarP(&downloadResolutionFlag,
		"resolution",
		"r",
		"best",
		"The video resolution. Can either be specified via the pixels, the abbreviation for pixels, or 'common-use' words\n"+
			"\tAvailable pixels: 1920x1080, 1280x720, 640x480, 480x360, 428x240\n"+
			"\tAvailable abbreviations: 1080p, 720p, 480p, 360p, 240p\n"+
			"\tAvailable common-use words: best (best available resolution), worst (worst available resolution)")

	Cmd.Flags().IntVarP(&downloadGoroutinesFlag,
		"goroutines",
		"g",
		runtime.NumCPU(),
		"Sets how many parallel segment downloads should be used")
}

func download(urls []string) error {
	for i, url := range urls {
		utils.Log.SetProcess("Parsing url %d", i+1)
		episodes, err := downloadExtractEpisodes(url)
		if err != nil {
			utils.Log.StopProcess("Failed to parse url %d", i+1)
			if utils.Crunchy.Config.Premium {
				utils.Log.Debug("If the error says no episodes could be found but the passed url is correct and a crunchyroll classic url, " +
					"try the corresponding crunchyroll beta url instead and try again. See https://github.com/crunchy-labs/crunchy-cli/issues/22 for more information")
			}
			return err
		}
		utils.Log.StopProcess("Parsed url %d", i+1)

		for _, season := range episodes {
			utils.Log.Info("%s Season %d", season[0].SeriesName, season[0].SeasonNumber)

			for j, info := range season {
				utils.Log.Info("\t%d. %s » %spx, %.2f FPS (S%02dE%02d)",
					j+1,
					info.Title,
					info.Resolution,
					info.FPS,
					info.SeasonNumber,
					info.EpisodeNumber)
			}
		}
		utils.Log.Empty()

		for j, season := range episodes {
			for k, info := range season {
				dir := info.FormatString(downloadDirectoryFlag)
				if _, err = os.Stat(dir); os.IsNotExist(err) {
					if err = os.MkdirAll(dir, 0777); err != nil {
						return fmt.Errorf("error while creating directory: %v", err)
					}
				}
				file, err := os.Create(utils.GenerateFilename(info.FormatString(downloadOutputFlag), dir))
				if err != nil {
					return fmt.Errorf("failed to create output file: %v", err)
				}

				if err = downloadInfo(info, file); err != nil {
					file.Close()
					os.Remove(file.Name())
					return err
				}
				file.Close()

				if i != len(urls)-1 || j != len(episodes)-1 || k != len(season)-1 {
					utils.Log.Empty()
				}
			}
		}
	}
	return nil
}

func downloadInfo(info utils.FormatInformation, file *os.File) error {
	utils.Log.Debug("Entering season %d, episode %d", info.SeasonNumber, info.EpisodeNumber)

	if err := info.Format.InitVideo(); err != nil {
		return fmt.Errorf("error while initializing the video: %v", err)
	}

	dp := &commands.DownloadProgress{
		Prefix:  utils.Log.(*commands.Logger).InfoLog.Prefix(),
		Message: "Downloading video",
		// number of segments a video has +2 is for merging and the success message
		Total: int(info.Format.Video.Chunklist.Count()) + 2,
		Dev:   utils.Log.IsDev(),
		Quiet: utils.Log.(*commands.Logger).IsQuiet(),
	}
	if utils.Log.IsDev() {
		dp.Prefix = utils.Log.(*commands.Logger).DebugLog.Prefix()
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

		if utils.Log.IsDev() {
			dp.UpdateMessage(fmt.Sprintf("Downloading %d/%d (%.2f%%) » %s", current, total, float32(current)/float32(total)*100, segment.URI), false)
		} else {
			dp.Update()
		}

		if current == total {
			dp.UpdateMessage("Merging segments", false)
		}
		return nil
	})
	tmp, _ := os.MkdirTemp(downloadTempDirFlag, "crunchy_")
	downloader.TempDir = tmp
	if utils.HasFFmpeg() {
		downloader.FFmpegOpts = make([]string, 0)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		select {
		case <-sig:
			signal.Stop(sig)
			utils.Log.Err("Exiting... (may take a few seconds)")
			utils.Log.Err("To force exit press ctrl+c (again)")
			cancel()
			// os.Exit(1) is not called because an immediate exit after the cancel function does not let
			// the download process enough time to stop gratefully. A result of this is that the temporary
			// directory where the segments are downloaded to will not be deleted
		case <-ctx.Done():
			// this is just here to end the goroutine and prevent it from running forever without a reason
		}
	}()
	utils.Log.Debug("Set up signal catcher")

	utils.Log.Info("Downloading episode `%s` to `%s`", info.Title, filepath.Base(file.Name()))
	utils.Log.Info("\tEpisode: S%02dE%02d", info.SeasonNumber, info.EpisodeNumber)
	utils.Log.Info("\tAudio: %s", info.Audio)
	utils.Log.Info("\tSubtitle: %s", info.Subtitle)
	utils.Log.Info("\tResolution: %spx", info.Resolution)
	utils.Log.Info("\tFPS: %.2f", info.FPS)
	if err := info.Format.Download(downloader); err != nil {
		return fmt.Errorf("error while downloading: %v", err)
	}

	dp.UpdateMessage("Download finished", false)

	signal.Stop(sig)
	utils.Log.Debug("Stopped signal catcher")

	utils.Log.Empty()

	return nil
}

func downloadExtractEpisodes(url string) ([][]utils.FormatInformation, error) {
	episodes, err := utils.ExtractEpisodes(url)
	if err != nil {
		return nil, err
	}

	var infoFormat [][]utils.FormatInformation
	for _, season := range crunchyUtils.SortEpisodesBySeason(episodes[0]) {
		tmpFormatInformation := make([]utils.FormatInformation, 0)
		for _, episode := range season {
			format, err := episode.GetFormat(downloadResolutionFlag, crunchyroll.LOCALE(downloadSubtitleFlag), true)
			if err != nil {
				return nil, fmt.Errorf("error while receiving format for %s: %v", episode.Title, err)
			}
			tmpFormatInformation = append(tmpFormatInformation, utils.FormatInformation{
				Format: format,

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
