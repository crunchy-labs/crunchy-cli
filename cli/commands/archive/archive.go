package archive

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/ByteDream/crunchy-cli/cli/commands"
	"github.com/ByteDream/crunchy-cli/utils"
	"github.com/ByteDream/crunchyroll-go/v3"
	crunchyUtils "github.com/ByteDream/crunchyroll-go/v3/utils"
	"github.com/grafov/m3u8"
	"github.com/spf13/cobra"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var (
	archiveLanguagesFlag []string

	archiveDirectoryFlag string
	archiveOutputFlag    string

	archiveMergeFlag string

	archiveCompressFlag string

	archiveResolutionFlag string

	archiveGoroutinesFlag int
)

var Cmd = &cobra.Command{
	Use:   "archive",
	Short: "Stores the given videos with all subtitles and multiple audios in a .mkv file",
	Args:  cobra.MinimumNArgs(1),

	PreRunE: func(cmd *cobra.Command, args []string) error {
		utils.Log.Debug("Validating arguments")

		if !utils.HasFFmpeg() {
			return fmt.Errorf("ffmpeg is needed to run this command correctly")
		}
		utils.Log.Debug("FFmpeg detected")

		if filepath.Ext(archiveOutputFlag) != ".mkv" {
			return fmt.Errorf("currently only matroska / .mkv files are supported")
		}

		for _, locale := range archiveLanguagesFlag {
			if !crunchyUtils.ValidateLocale(crunchyroll.LOCALE(locale)) {
				// if locale is 'all', match all known locales
				if locale == "all" {
					archiveLanguagesFlag = utils.LocalesAsStrings()
					break
				}
				return fmt.Errorf("%s is not a valid locale. Choose from: %s", locale, strings.Join(utils.LocalesAsStrings(), ", "))
			}
		}
		utils.Log.Debug("Using following audio locales: %s", strings.Join(archiveLanguagesFlag, ", "))

		var found bool
		for _, mode := range []string{"auto", "audio", "video"} {
			if archiveMergeFlag == mode {
				utils.Log.Debug("Using %s merge behavior", archiveMergeFlag)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("'%s' is no valid merge flag. Use 'auto', 'audio' or 'video'", archiveMergeFlag)
		}

		if archiveCompressFlag != "" {
			found = false
			for _, algo := range []string{".tar", ".tar.gz", ".tgz", ".zip"} {
				if strings.HasSuffix(archiveCompressFlag, algo) {
					utils.Log.Debug("Using %s compression", algo)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("'%s' is no valid compress algorithm. Valid algorithms / file endings are '.tar', '.tar.gz', '.zip'",
					archiveCompressFlag)
			}
		}

		switch archiveResolutionFlag {
		case "1080p", "720p", "480p", "360p":
			intRes, _ := strconv.ParseFloat(strings.TrimSuffix(archiveResolutionFlag, "p"), 84)
			archiveResolutionFlag = fmt.Sprintf("%.0fx%s", math.Ceil(intRes*(float64(16)/float64(9))), strings.TrimSuffix(archiveResolutionFlag, "p"))
		case "240p":
			// 240p would round up to 427x240 if used in the case statement above, so it has to be handled separately
			archiveResolutionFlag = "428x240"
		case "1920x1080", "1280x720", "640x480", "480x360", "428x240", "best", "worst":
		default:
			return fmt.Errorf("'%s' is not a valid resolution", archiveResolutionFlag)
		}
		utils.Log.Debug("Using resolution '%s'", archiveResolutionFlag)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := commands.LoadCrunchy(); err != nil {
			return err
		}

		return archive(args)
	},
}

func init() {
	Cmd.Flags().StringSliceVarP(&archiveLanguagesFlag,
		"language",
		"l",
		[]string{string(utils.SystemLocale(false)), string(crunchyroll.JP)},
		"Audio locale which should be downloaded. Can be used multiple times")

	cwd, _ := os.Getwd()
	Cmd.Flags().StringVarP(&archiveDirectoryFlag,
		"directory",
		"d",
		cwd,
		"The directory to store the files into")
	Cmd.Flags().StringVarP(&archiveOutputFlag,
		"output",
		"o",
		"{title}.mkv",
		"Name of the output file. If you use the following things in the name, the will get replaced:\n"+
			"\t{title} » Title of the video\n"+
			"\t{series_name} » Name of the series\n"+
			"\t{season_name} » Name of the season\n"+
			"\t{season_number} » Number of the season\n"+
			"\t{episode_number} » Number of the episode\n"+
			"\t{resolution} » Resolution of the video\n"+
			"\t{fps} » Frame Rate of the video\n"+
			"\t{audio} » Audio locale of the video\n"+
			"\t{subtitle} » Subtitle locale of the video")

	Cmd.Flags().StringVarP(&archiveMergeFlag,
		"merge",
		"m",
		"auto",
		"Sets the behavior of the stream merging. Valid behaviors are 'auto', 'audio', 'video'")

	Cmd.Flags().StringVarP(&archiveCompressFlag,
		"compress",
		"c",
		"",
		"If is set, all output will be compresses into an archive (every url generates a new one). "+
			"This flag sets the name of the compressed output file. The file ending specifies the compression algorithm. "+
			"The following algorithms are supported: gzip, tar, zip")

	Cmd.Flags().StringVarP(&archiveResolutionFlag,
		"resolution",
		"r",
		"best",
		"The video resolution. Can either be specified via the pixels, the abbreviation for pixels, or 'common-use' words\n"+
			"\tAvailable pixels: 1920x1080, 1280x720, 640x480, 480x360, 428x240\n"+
			"\tAvailable abbreviations: 1080p, 720p, 480p, 360p, 240p\n"+
			"\tAvailable common-use words: best (best available resolution), worst (worst available resolution)")

	Cmd.Flags().IntVarP(&archiveGoroutinesFlag,
		"goroutines",
		"g",
		runtime.NumCPU(),
		"Number of parallel segment downloads")
}

func archive(urls []string) error {
	for i, url := range urls {
		utils.Log.SetProcess("Parsing url %d", i+1)
		episodes, err := archiveExtractEpisodes(url)
		if err != nil {
			utils.Log.StopProcess("Failed to parse url %d", i+1)
			if utils.Crunchy.Config.Premium {
				utils.Log.Debug("If the error says no episodes could be found but the passed url is correct and a crunchyroll classic url, " +
					"try the corresponding crunchyroll beta url instead and try again. See https://github.com/ByteDream/crunchy-cli/issues/22 for more information")
			}
			return err
		}
		utils.Log.StopProcess("Parsed url %d", i+1)

		var compressFile *os.File
		var c Compress

		if archiveCompressFlag != "" {
			compressFile, err = os.Create(utils.GenerateFilename(archiveCompressFlag, ""))
			if err != nil {
				return fmt.Errorf("failed to create archive file: %v", err)
			}
			if strings.HasSuffix(archiveCompressFlag, ".tar") {
				c = NewTarCompress(compressFile)
			} else if strings.HasSuffix(archiveCompressFlag, ".tar.gz") || strings.HasSuffix(archiveCompressFlag, ".tgz") {
				c = NewGzipCompress(compressFile)
			} else if strings.HasSuffix(archiveCompressFlag, ".zip") {
				c = NewZipCompress(compressFile)
			}
		}

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
				var filename string
				var writeCloser io.WriteCloser
				if c != nil {
					filename = info.FormatString(archiveOutputFlag)
					writeCloser, err = c.NewFile(info)
					if err != nil {
						return fmt.Errorf("failed to pre generate new archive file: %v", err)
					}
				} else {
					dir := info.FormatString(archiveDirectoryFlag)
					if _, err = os.Stat(dir); os.IsNotExist(err) {
						if err = os.MkdirAll(dir, 0777); err != nil {
							return fmt.Errorf("error while creating directory: %v", err)
						}
					}
					filename = utils.GenerateFilename(info.FormatString(archiveOutputFlag), dir)
					writeCloser, err = os.Create(filename)
					if err != nil {
						return fmt.Errorf("failed to create new file: %v", err)
					}
				}

				if err = archiveInfo(info, writeCloser, filename); err != nil {
					writeCloser.Close()
					if f, ok := writeCloser.(*os.File); ok {
						os.Remove(f.Name())
					} else {
						c.Close()
						compressFile.Close()
						os.RemoveAll(compressFile.Name())
					}
					return err
				}
				writeCloser.Close()

				if i != len(urls)-1 || j != len(episodes)-1 || k != len(season)-1 {
					utils.Log.Empty()
				}
			}
		}
		if c != nil {
			c.Close()
		}
		if compressFile != nil {
			compressFile.Close()
		}
	}
	return nil
}

func archiveInfo(info utils.FormatInformation, writeCloser io.WriteCloser, filename string) error {
	utils.Log.Debug("Entering season %d, episode %d with %d additional formats", info.SeasonNumber, info.EpisodeNumber, len(info.AdditionalFormats))

	dp, err := createArchiveProgress(info)
	if err != nil {
		return fmt.Errorf("error while setting up downloader: %v", err)
	}
	defer func() {
		if dp.Total != dp.Current {
			fmt.Println()
		}
	}()

	rootFile, err := os.CreateTemp("", fmt.Sprintf("%s_*.ts", strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(rootFile.Name())
	defer rootFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	downloader := crunchyroll.NewDownloader(ctx, rootFile, archiveGoroutinesFlag, func(segment *m3u8.MediaSegment, current, total int, file *os.File) error {
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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		select {
		case <-sig:
			signal.Stop(sig)
			utils.Log.Err("Exiting... (may take a few seconds)")
			utils.Log.Err("To force exit press ctrl+c (again)")
			cancel()
			// os.Exit(1) is not called since an immediate exit after the cancel function does not let
			// the download process enough time to stop gratefully. A result of this is that the temporary
			// directory where the segments are downloaded to will not be deleted
		case <-ctx.Done():
			// this is just here to end the goroutine and prevent it from running forever without a reason
		}
	}()
	utils.Log.Debug("Set up signal catcher")

	var additionalDownloaderOpts []string
	var mergeMessage string
	switch archiveMergeFlag {
	case "auto":
		additionalDownloaderOpts = []string{"-vn"}
		for _, format := range info.AdditionalFormats {
			if format.Video.Bandwidth != info.Format.Video.Bandwidth {
				// revoke the changed FFmpegOpts above
				additionalDownloaderOpts = []string{}
				break
			}
		}
		if len(additionalDownloaderOpts) > 0 {
			mergeMessage = "merging audio for additional formats"
		} else {
			mergeMessage = "merging video for additional formats"
		}
	case "audio":
		additionalDownloaderOpts = []string{"-vn"}
		mergeMessage = "merging audio for additional formats"
	case "video":
		mergeMessage = "merging video for additional formats"
	}

	utils.Log.Info("Downloading episode `%s` to `%s` (%s)", info.Title, filepath.Base(filename), mergeMessage)
	utils.Log.Info("\tEpisode: S%02dE%02d", info.SeasonNumber, info.EpisodeNumber)
	utils.Log.Info("\tAudio: %s", info.Audio)
	utils.Log.Info("\tSubtitle: %s", info.Subtitle)
	utils.Log.Info("\tResolution: %spx", info.Resolution)
	utils.Log.Info("\tFPS: %.2f", info.FPS)

	var videoFiles, audioFiles, subtitleFiles []string
	defer func() {
		for _, f := range append(append(videoFiles, audioFiles...), subtitleFiles...) {
			os.RemoveAll(f)
		}
	}()

	var f []string
	if f, err = archiveDownloadVideos(downloader, filepath.Base(filename), true, info.Format); err != nil {
		if err != ctx.Err() {
			return fmt.Errorf("error while downloading: %v", err)
		}
		return err
	}
	videoFiles = append(videoFiles, f[0])

	if len(additionalDownloaderOpts) == 0 {
		var videos []string
		downloader.FFmpegOpts = additionalDownloaderOpts
		if videos, err = archiveDownloadVideos(downloader, filepath.Base(filename), true, info.AdditionalFormats...); err != nil {
			return fmt.Errorf("error while downloading additional videos: %v", err)
		}
		downloader.FFmpegOpts = []string{}
		videoFiles = append(videoFiles, videos...)
	} else {
		var audios []string
		if audios, err = archiveDownloadVideos(downloader, filepath.Base(filename), false, info.AdditionalFormats...); err != nil {
			return fmt.Errorf("error while downloading additional videos: %v", err)
		}
		audioFiles = append(audioFiles, audios...)
	}

	sort.Sort(crunchyUtils.SubtitlesByLocale(info.Format.Subtitles))

	sortSubtitles, _ := strconv.ParseBool(os.Getenv("SORT_SUBTITLES"))
	if sortSubtitles && len(archiveLanguagesFlag) > 0 {
		// this sort the subtitle locales after the languages which were specified
		// with the `archiveLanguagesFlag` flag
		for _, language := range archiveLanguagesFlag {
			for i, subtitle := range info.Format.Subtitles {
				if subtitle.Locale == crunchyroll.LOCALE(language) {
					info.Format.Subtitles = append([]*crunchyroll.Subtitle{subtitle}, append(info.Format.Subtitles[:i], info.Format.Subtitles[i+1:]...)...)
					break
				}
			}
		}
	}

	var subtitles []string
	if subtitles, err = archiveDownloadSubtitles(filepath.Base(filename), info.Format.Subtitles...); err != nil {
		return fmt.Errorf("error while downloading subtitles: %v", err)
	}
	subtitleFiles = append(subtitleFiles, subtitles...)

	if err = archiveFFmpeg(ctx, writeCloser, videoFiles, audioFiles, subtitleFiles); err != nil {
		return fmt.Errorf("failed to merge files: %v", err)
	}

	dp.UpdateMessage("Download finished", false)

	signal.Stop(sig)
	utils.Log.Debug("Stopped signal catcher")

	utils.Log.Empty()

	return nil
}

func createArchiveProgress(info utils.FormatInformation) (*commands.DownloadProgress, error) {
	var progressCount int
	if err := info.Format.InitVideo(); err != nil {
		return nil, fmt.Errorf("error while initializing a video: %v", err)
	}
	// + number of segments a video has +1 is for merging
	progressCount += int(info.Format.Video.Chunklist.Count()) + 1
	for _, f := range info.AdditionalFormats {
		if f == info.Format {
			continue
		}

		if err := f.InitVideo(); err != nil {
			return nil, err
		}
		// + number of segments a video has +1 is for merging
		progressCount += int(f.Video.Chunklist.Count()) + 1
	}

	dp := &commands.DownloadProgress{
		Prefix:  utils.Log.(*commands.Logger).InfoLog.Prefix(),
		Message: "Downloading video",
		// number of segments a video +1 is for the success message
		Total: progressCount + 1,
		Dev:   utils.Log.IsDev(),
		Quiet: utils.Log.(*commands.Logger).IsQuiet(),
	}
	if utils.Log.IsDev() {
		dp.Prefix = utils.Log.(*commands.Logger).DebugLog.Prefix()
	}

	return dp, nil
}

func archiveDownloadVideos(downloader crunchyroll.Downloader, filename string, video bool, formats ...*crunchyroll.Format) ([]string, error) {
	var files []string

	for _, format := range formats {
		var name string
		if video {
			name = fmt.Sprintf("%s_%s_video_*.ts", filename, format.AudioLocale)
		} else {
			name = fmt.Sprintf("%s_%s_audio_*.aac", filename, format.AudioLocale)
		}

		f, err := os.CreateTemp("", name)
		if err != nil {
			return nil, err
		}
		files = append(files, f.Name())

		downloader.Writer = f
		if err = format.Download(downloader); err != nil {
			f.Close()
			for _, file := range files {
				os.Remove(file)
			}
			return nil, err
		}
		f.Close()

		utils.Log.Debug("Downloaded '%s' video", format.AudioLocale)
	}

	return files, nil
}

func archiveDownloadSubtitles(filename string, subtitles ...*crunchyroll.Subtitle) ([]string, error) {
	var files []string

	for _, subtitle := range subtitles {
		f, err := os.CreateTemp("", fmt.Sprintf("%s_%s_subtitle_*.ass", filename, subtitle.Locale))
		if err != nil {
			return nil, err
		}
		files = append(files, f.Name())

		if err := subtitle.Save(f); err != nil {
			f.Close()
			for _, file := range files {
				os.Remove(file)
			}
			return nil, err
		}
		f.Close()

		utils.Log.Debug("Downloaded '%s' subtitles", subtitle.Locale)
	}

	return files, nil
}

func archiveFFmpeg(ctx context.Context, dst io.Writer, videoFiles, audioFiles, subtitleFiles []string) error {
	var input, maps, metadata []string
	re := regexp.MustCompile(`(?m)_([a-z]{2}-([A-Z]{2}|[0-9]{3}))_(video|audio|subtitle)`)

	for i, video := range videoFiles {
		input = append(input, "-i", video)
		maps = append(maps, "-map", strconv.Itoa(i))
		locale := crunchyroll.LOCALE(re.FindStringSubmatch(video)[1])
		metadata = append(metadata, fmt.Sprintf("-metadata:s:v:%d", i), fmt.Sprintf("language=%s", locale))
		metadata = append(metadata, fmt.Sprintf("-metadata:s:v:%d", i), fmt.Sprintf("title=%s", crunchyUtils.LocaleLanguage(locale)))
		metadata = append(metadata, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", locale))
		metadata = append(metadata, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("title=%s", crunchyUtils.LocaleLanguage(locale)))
	}

	for i, audio := range audioFiles {
		input = append(input, "-i", audio)
		maps = append(maps, "-map", strconv.Itoa(i+len(videoFiles)))
		locale := crunchyroll.LOCALE(re.FindStringSubmatch(audio)[1])
		metadata = append(metadata, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", locale))
		metadata = append(metadata, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("title=%s", crunchyUtils.LocaleLanguage(locale)))
	}

	for i, subtitle := range subtitleFiles {
		input = append(input, "-i", subtitle)
		maps = append(maps, "-map", strconv.Itoa(i+len(videoFiles)+len(audioFiles)))
		locale := crunchyroll.LOCALE(re.FindStringSubmatch(subtitle)[1])
		metadata = append(metadata, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("language=%s", locale))
		metadata = append(metadata, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("title=%s", crunchyUtils.LocaleLanguage(locale)))
	}

	commandOptions := []string{"-y"}
	commandOptions = append(commandOptions, input...)
	commandOptions = append(commandOptions, maps...)
	commandOptions = append(commandOptions, metadata...)
	// we have to create a temporary file here because it must be seekable
	// for ffmpeg.
	// ffmpeg could write to dst too, but this would require to re-encode
	// the audio which results in much higher time and resource consumption
	// (0-1 second with the temp file, ~20 seconds with re-encoding on my system)
	file, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	file.Close()
	defer os.Remove(file.Name())

	commandOptions = append(commandOptions, "-disposition:s:0", "0", "-c", "copy", "-f", "matroska", file.Name())

	// just a little nicer debug output to copy and paste the ffmpeg for debug reasons
	if utils.Log.IsDev() {
		var debugOptions []string

		for _, option := range commandOptions {
			if strings.HasPrefix(option, "title=") {
				debugOptions = append(debugOptions, "title=\""+strings.TrimPrefix(option, "title=")+"\"")
			} else if strings.HasPrefix(option, "language=") {
				debugOptions = append(debugOptions, "language=\""+strings.TrimPrefix(option, "language=")+"\"")
			} else if strings.Contains(option, " ") {
				debugOptions = append(debugOptions, "\""+option+"\"")
			} else {
				debugOptions = append(debugOptions, option)
			}
		}
		utils.Log.Debug("FFmpeg merge command: ffmpeg %s", strings.Join(debugOptions, " "))
	}

	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", commandOptions...)
	cmd.Stderr = &errBuf
	if err = cmd.Run(); err != nil {
		return fmt.Errorf(errBuf.String())
	}

	file, err = os.Open(file.Name())
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = bufio.NewWriter(dst).ReadFrom(file)
	return err
}

func archiveExtractEpisodes(url string) ([][]utils.FormatInformation, error) {
	var hasJapanese bool
	languagesAsLocale := []crunchyroll.LOCALE{crunchyroll.JP}
	for _, language := range archiveLanguagesFlag {
		locale := crunchyroll.LOCALE(language)
		if locale == crunchyroll.JP {
			hasJapanese = true
		} else {
			languagesAsLocale = append(languagesAsLocale, locale)
		}
	}

	episodes, err := utils.ExtractEpisodes(url, languagesAsLocale...)
	if err != nil {
		return nil, err
	}

	if !hasJapanese && len(episodes[1:]) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	for i, eps := range episodes {
		if len(eps) == 0 {
			utils.Log.SetProcess("%s has no matching episodes", languagesAsLocale[i])
		} else if len(episodes[0]) > len(eps) {
			utils.Log.SetProcess("%s has %d less episodes than existing in japanese (%d)", languagesAsLocale[i], len(episodes[0])-len(eps), len(episodes[0]))
		}
	}

	if !hasJapanese {
		episodes = episodes[1:]
	}

	eps := make(map[int]map[int]*utils.FormatInformation)
	for _, lang := range episodes {
		for _, season := range crunchyUtils.SortEpisodesBySeason(lang) {
			if _, ok := eps[season[0].SeasonNumber]; !ok {
				eps[season[0].SeasonNumber] = map[int]*utils.FormatInformation{}
			}
			for _, episode := range season {
				format, err := episode.GetFormat(archiveResolutionFlag, "", false)
				if err != nil {
					return nil, fmt.Errorf("error while receiving format for %s: %v", episode.Title, err)
				}

				if _, ok := eps[episode.SeasonNumber][episode.EpisodeNumber]; !ok {
					eps[episode.SeasonNumber][episode.EpisodeNumber] = &utils.FormatInformation{
						Format:            format,
						AdditionalFormats: make([]*crunchyroll.Format, 0),

						Title:         episode.Title,
						SeriesName:    episode.SeriesTitle,
						SeasonName:    episode.SeasonTitle,
						SeasonNumber:  episode.SeasonNumber,
						EpisodeNumber: episode.EpisodeNumber,
						Resolution:    format.Video.Resolution,
						FPS:           format.Video.FrameRate,
						Audio:         format.AudioLocale,
					}
				} else {
					eps[episode.SeasonNumber][episode.EpisodeNumber].AdditionalFormats = append(eps[episode.SeasonNumber][episode.EpisodeNumber].AdditionalFormats, format)
				}
			}
		}
	}

	var infoFormat [][]utils.FormatInformation
	for _, e := range eps {
		var tmpFormatInfo []utils.FormatInformation

		var keys []int
		for episodeNumber := range e {
			keys = append(keys, episodeNumber)
		}
		sort.Ints(keys)

		for _, key := range keys {
			tmpFormatInfo = append(tmpFormatInfo, *e[key])
		}

		infoFormat = append(infoFormat, tmpFormatInfo)
	}

	return infoFormat, nil
}
