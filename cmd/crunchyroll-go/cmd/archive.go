package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/ByteDream/crunchyroll-go/utils"
	"github.com/grafov/m3u8"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Stores the given videos with all subtitles and multiple audios in a .mkv file",
	Args:  cobra.MinimumNArgs(1),

	PreRunE: func(cmd *cobra.Command, args []string) error {
		out.Debug("Validating arguments")

		if !hasFFmpeg() {
			return fmt.Errorf("ffmpeg is needed to run this command correctly")
		}
		out.Debug("FFmpeg detected")

		if filepath.Ext(archiveOutputFlag) != ".mkv" {
			return fmt.Errorf("currently only matroska / .mkv files are supported")
		}

		for _, locale := range archiveLanguagesFlag {
			if !utils.ValidateLocale(crunchyroll.LOCALE(locale)) {
				// if locale is 'all', match all known locales
				if locale == "all" {
					archiveLanguagesFlag = allLocalesAsStrings()
					break
				}
				return fmt.Errorf("%s is not a valid locale. Choose from: %s", locale, strings.Join(allLocalesAsStrings(), ", "))
			}
		}
		out.Debug("Using following audio locales: %s", strings.Join(archiveLanguagesFlag, ", "))

		var found bool
		for _, mode := range []string{"auto", "audio", "video"} {
			if archiveMergeFlag == mode {
				out.Debug("Using %s merge behavior", archiveMergeFlag)
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
					out.Debug("Using %s compression", algo)
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
		case "1080p", "720p", "480p", "360p", "240p":
			intRes, _ := strconv.ParseFloat(strings.TrimSuffix(archiveResolutionFlag, "p"), 84)
			archiveResolutionFlag = fmt.Sprintf("%dx%s", int(intRes*(16/9)), strings.TrimSuffix(archiveResolutionFlag, "p"))
		case "1920x1080", "1280x720", "640x480", "480x360", "428x240", "best", "worst":
		default:
			return fmt.Errorf("'%s' is not a valid resolution", archiveResolutionFlag)
		}
		out.Debug("Using resolution '%s'", archiveResolutionFlag)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		loadCrunchy()

		return archive(args)
	},
}

func init() {
	archiveCmd.Flags().StringSliceVarP(&archiveLanguagesFlag,
		"language",
		"l",
		[]string{string(systemLocale(false)), string(crunchyroll.JP)},
		"Audio locale which should be downloaded. Can be used multiple times")

	cwd, _ := os.Getwd()
	archiveCmd.Flags().StringVarP(&archiveDirectoryFlag,
		"directory",
		"d",
		cwd,
		"The directory to store the files into")
	archiveCmd.Flags().StringVarP(&archiveOutputFlag,
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

	archiveCmd.Flags().StringVarP(&archiveMergeFlag,
		"merge",
		"m",
		"auto",
		"Sets the behavior of the stream merging. Valid behaviors are 'auto', 'audio', 'video'")

	archiveCmd.Flags().StringVarP(&archiveCompressFlag,
		"compress",
		"c",
		"",
		"If is set, all output will be compresses into an archive (every url generates a new one). "+
			"This flag sets the name of the compressed output file. The file ending specifies the compression algorithm. "+
			"The following algorithms are supported: gzip, tar, zip")

	archiveCmd.Flags().StringVarP(&archiveResolutionFlag,
		"resolution",
		"r",
		"best",
		"The video resolution. Can either be specified via the pixels, the abbreviation for pixels, or 'common-use' words\n"+
			"\tAvailable pixels: 1920x1080, 1280x720, 640x480, 480x360, 428x240\n"+
			"\tAvailable abbreviations: 1080p, 720p, 480p, 360p, 240p\n"+
			"\tAvailable common-use words: best (best available resolution), worst (worst available resolution)")

	archiveCmd.Flags().IntVarP(&archiveGoroutinesFlag,
		"goroutines",
		"g",
		runtime.NumCPU(),
		"Number of parallel segment downloads")

	rootCmd.AddCommand(archiveCmd)
}

func archive(urls []string) error {
	for i, url := range urls {
		out.SetProgress("Parsing url %d", i+1)
		episodes, err := archiveExtractEpisodes(url)
		if err != nil {
			out.StopProgress("Failed to parse url %d", i+1)
			return err
		}
		out.StopProgress("Parsed url %d", i+1)

		var compressFile *os.File
		var c compress

		if archiveCompressFlag != "" {
			compressFile, err = os.Create(generateFilename(archiveCompressFlag, ""))
			if err != nil {
				return fmt.Errorf("failed to create archive file: %v", err)
			}
			if strings.HasSuffix(archiveCompressFlag, ".tar") {
				c = newTarCompress(compressFile)
			} else if strings.HasSuffix(archiveCompressFlag, ".tar.gz") || strings.HasSuffix(archiveCompressFlag, ".tgz") {
				c = newGzipCompress(compressFile)
			} else if strings.HasSuffix(archiveCompressFlag, ".zip") {
				c = newZipCompress(compressFile)
			}
		}

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
				var filename string
				var writeCloser io.WriteCloser
				if c != nil {
					filename = info.Format(archiveOutputFlag)
					writeCloser, err = c.NewFile(info)
					if err != nil {
						return fmt.Errorf("failed to pre generate new archive file: %v", err)
					}
				} else {
					dir := info.Format(archiveDirectoryFlag)
					if _, err = os.Stat(dir); os.IsNotExist(err) {
						if err = os.MkdirAll(dir, 0777); err != nil {
							return fmt.Errorf("error while creating directory: %v", err)
						}
					}
					filename = generateFilename(info.Format(archiveOutputFlag), dir)
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

func archiveInfo(info formatInformation, writeCloser io.WriteCloser, filename string) error {
	out.Debug("Entering season %d, episode %d with %d additional formats", info.SeasonNumber, info.EpisodeNumber, len(info.additionalFormats))

	downloadProgress, err := createArchiveProgress(info)
	if err != nil {
		return fmt.Errorf("error while setting up downloader: %v", err)
	}
	defer func() {
		if downloadProgress.Total != downloadProgress.Current {
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

		if out.IsDev() {
			downloadProgress.UpdateMessage(fmt.Sprintf("Downloading %d/%d (%.2f%%) » %s", current, total, float32(current)/float32(total)*100, segment.URI), false)
		} else {
			downloadProgress.Update()
		}

		if current == total {
			downloadProgress.UpdateMessage("Merging segments", false)
		}
		return nil
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		select {
		case <-sig:
			signal.Stop(sig)
			out.Exit("Exiting... (may take a few seconds)")
			out.Exit("To force exit press ctrl+c (again)")
			cancel()
			// os.Exit(1) is not called since an immediate exit after the cancel function does not let
			// the download process enough time to stop gratefully. A result of this is that the temporary
			// directory where the segments are downloaded to will not be deleted
		case <-ctx.Done():
			// this is just here to end the goroutine and prevent it from running forever without a reason
		}
	}()
	out.Debug("Set up signal catcher")

	var additionalDownloaderOpts []string
	var mergeMessage string
	switch archiveMergeFlag {
	case "auto":
		additionalDownloaderOpts = []string{"-vn"}
		for _, format := range info.additionalFormats {
			if format.Video.Bandwidth != info.format.Video.Bandwidth {
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

	out.Info("Downloading episode `%s` to `%s` (%s)", info.Title, filepath.Base(filename), mergeMessage)
	out.Info("\tEpisode: S%02dE%02d", info.SeasonNumber, info.EpisodeNumber)
	out.Info("\tAudio: %s", info.Audio)
	out.Info("\tSubtitle: %s", info.Subtitle)
	out.Info("\tResolution: %spx", info.Resolution)
	out.Info("\tFPS: %.2f", info.FPS)

	var videoFiles, audioFiles, subtitleFiles []string
	defer func() {
		for _, f := range append(append(videoFiles, audioFiles...), subtitleFiles...) {
			os.RemoveAll(f)
		}
	}()

	var f []string
	if f, err = archiveDownloadVideos(downloader, filepath.Base(filename), true, info.format); err != nil {
		if err != ctx.Err() {
			return fmt.Errorf("error while downloading: %v", err)
		}
		return err
	}
	videoFiles = append(videoFiles, f[0])

	if len(additionalDownloaderOpts) == 0 {
		var videos []string
		downloader.FFmpegOpts = additionalDownloaderOpts
		if videos, err = archiveDownloadVideos(downloader, filepath.Base(filename), true, info.additionalFormats...); err != nil {
			return fmt.Errorf("error while downloading additional videos: %v", err)
		}
		downloader.FFmpegOpts = []string{}
		videoFiles = append(videoFiles, videos...)
	} else {
		var audios []string
		if audios, err = archiveDownloadVideos(downloader, filepath.Base(filename), false, info.additionalFormats...); err != nil {
			return fmt.Errorf("error while downloading additional videos: %v", err)
		}
		audioFiles = append(audioFiles, audios...)
	}

	sort.Sort(utils.SubtitlesByLocale(info.format.Subtitles))

	sortSubtitles, _ := strconv.ParseBool(os.Getenv("SORT_SUBTITLES"))
	if sortSubtitles && len(archiveLanguagesFlag) > 0 {
		// this sort the subtitle locales after the languages which were specified
		// with the `archiveLanguagesFlag` flag
		for _, language := range archiveLanguagesFlag {
			for i, subtitle := range info.format.Subtitles {
				if subtitle.Locale == crunchyroll.LOCALE(language) {
					info.format.Subtitles = append([]*crunchyroll.Subtitle{subtitle}, append(info.format.Subtitles[:i], info.format.Subtitles[i+1:]...)...)
					break
				}
			}
		}
	}

	var subtitles []string
	if subtitles, err = archiveDownloadSubtitles(filepath.Base(filename), info.format.Subtitles...); err != nil {
		return fmt.Errorf("error while downloading subtitles: %v", err)
	}
	subtitleFiles = append(subtitleFiles, subtitles...)

	if err = archiveFFmpeg(ctx, writeCloser, videoFiles, audioFiles, subtitleFiles); err != nil {
		return fmt.Errorf("failed to merge files: %v", err)
	}

	downloadProgress.UpdateMessage("Download finished", false)

	signal.Stop(sig)
	out.Debug("Stopped signal catcher")

	out.Empty()
	out.Empty()

	return nil
}

func createArchiveProgress(info formatInformation) (*downloadProgress, error) {
	var progressCount int
	if err := info.format.InitVideo(); err != nil {
		return nil, fmt.Errorf("error while initializing a video: %v", err)
	}
	// + number of segments a video has +1 is for merging
	progressCount += int(info.format.Video.Chunklist.Count()) + 1
	for _, f := range info.additionalFormats {
		if f == info.format {
			continue
		}

		if err := f.InitVideo(); err != nil {
			return nil, err
		}
		// + number of segments a video has +1 is for merging
		progressCount += int(f.Video.Chunklist.Count()) + 1
	}

	downloadProgress := &downloadProgress{
		Prefix:  out.InfoLog.Prefix(),
		Message: "Downloading video",
		// number of segments a video +1 is for the success message
		Total: progressCount + 1,
		Dev:   out.IsDev(),
		Quiet: out.IsQuiet(),
	}
	if out.IsDev() {
		downloadProgress.Prefix = out.DebugLog.Prefix()
	}

	return downloadProgress, nil
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

		out.Debug("Downloaded '%s' video", format.AudioLocale)
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

		out.Debug("Downloaded '%s' subtitles", subtitle.Locale)
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
		metadata = append(metadata, fmt.Sprintf("-metadata:s:v:%d", i), fmt.Sprintf("language=%s", utils.LocaleLanguage(locale)))
		metadata = append(metadata, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", utils.LocaleLanguage(locale)))
	}

	for i, audio := range audioFiles {
		input = append(input, "-i", audio)
		maps = append(maps, "-map", strconv.Itoa(i+len(videoFiles)))
		locale := crunchyroll.LOCALE(re.FindStringSubmatch(audio)[1])
		metadata = append(metadata, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("language=%s", utils.LocaleLanguage(locale)))
	}

	for i, subtitle := range subtitleFiles {
		input = append(input, "-i", subtitle)
		maps = append(maps, "-map", strconv.Itoa(i+len(videoFiles)+len(audioFiles)))
		locale := crunchyroll.LOCALE(re.FindStringSubmatch(subtitle)[1])
		metadata = append(metadata, fmt.Sprintf("-metadata:s:s:%d", i), fmt.Sprintf("title=%s", utils.LocaleLanguage(locale)))
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

	commandOptions = append(commandOptions, "-c", "copy", "-f", "matroska", file.Name())

	// just a little nicer debug output to copy and paste the ffmpeg for debug reasons
	if out.IsDev() {
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
		out.Debug("FFmpeg merge command: ffmpeg %s", strings.Join(debugOptions, " "))
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

func archiveExtractEpisodes(url string) ([][]formatInformation, error) {
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

	episodes, err := extractEpisodes(url, languagesAsLocale...)
	if err != nil {
		return nil, err
	}

	if !hasJapanese && len(episodes[1:]) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	for i, eps := range episodes {
		if len(eps) == 0 {
			out.SetProgress("%s has no matching episodes", languagesAsLocale[i])
		} else if len(episodes[0]) > len(eps) {
			out.SetProgress("%s has %d less episodes than existing in japanese (%d)", languagesAsLocale[i], len(episodes[0])-len(eps), len(episodes[0]))
		}
	}

	if !hasJapanese {
		episodes = episodes[1:]
	}

	eps := make(map[int]map[int]*formatInformation)
	for _, lang := range episodes {
		for _, season := range utils.SortEpisodesBySeason(lang) {
			if _, ok := eps[season[0].SeasonNumber]; !ok {
				eps[season[0].SeasonNumber] = map[int]*formatInformation{}
			}
			for _, episode := range season {
				format, err := episode.GetFormat(archiveResolutionFlag, "", false)
				if err != nil {
					return nil, fmt.Errorf("error while receiving format for %s: %v", episode.Title, err)
				}

				if _, ok := eps[episode.SeasonNumber][episode.EpisodeNumber]; !ok {
					eps[episode.SeasonNumber][episode.EpisodeNumber] = &formatInformation{
						format:            format,
						additionalFormats: make([]*crunchyroll.Format, 0),

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
					eps[episode.SeasonNumber][episode.EpisodeNumber].additionalFormats = append(eps[episode.SeasonNumber][episode.EpisodeNumber].additionalFormats, format)
				}
			}
		}
	}

	var infoFormat [][]formatInformation
	for _, e := range eps {
		var tmpFormatInfo []formatInformation

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

type compress interface {
	io.Closer

	NewFile(information formatInformation) (io.WriteCloser, error)
}

func newGzipCompress(file *os.File) *tarCompress {
	gw := gzip.NewWriter(file)
	return &tarCompress{
		parent: gw,
		dst:    tar.NewWriter(gw),
	}
}

func newTarCompress(file *os.File) *tarCompress {
	return &tarCompress{
		dst: tar.NewWriter(file),
	}
}

type tarCompress struct {
	compress

	wg sync.WaitGroup

	parent *gzip.Writer
	dst    *tar.Writer
}

func (tc *tarCompress) Close() error {
	// we have to wait here in case the actual content isn't copied completely into the
	// writer yet
	tc.wg.Wait()

	var err, err2 error
	if tc.parent != nil {
		err2 = tc.parent.Close()
	}
	err = tc.dst.Close()

	if err != nil && err2 != nil {
		// best way to show double errors at once that i've found
		return fmt.Errorf("%v\n%v", err, err2)
	} else if err == nil && err2 != nil {
		err = err2
	}

	return err
}

func (tc *tarCompress) NewFile(information formatInformation) (io.WriteCloser, error) {
	rp, wp := io.Pipe()
	go func() {
		tc.wg.Add(1)
		defer tc.wg.Done()
		var buf bytes.Buffer
		io.Copy(&buf, rp)

		header := &tar.Header{
			Name:     filepath.Join(fmt.Sprintf("S%2d", information.SeasonNumber), information.Title),
			ModTime:  time.Now(),
			Mode:     0644,
			Typeflag: tar.TypeReg,
			// fun fact: i did not set the size for quiet some time because i thought that it isn't
			// required. well because of this i debugged this part for multiple hours because without
			// proper size information only a tiny amount gets copied into the tar (or zip) writer.
			// this is also the reason why the file content is completely copied into a buffer before
			// writing it to the writer. i could bypass this and save some memory but this requires
			// some rewriting and im nearly at the (planned) finish for version 2 so nah in the future
			// maybe
			Size: int64(buf.Len()),
		}
		tc.dst.WriteHeader(header)
		io.Copy(tc.dst, &buf)
	}()
	return wp, nil
}

func newZipCompress(file *os.File) *zipCompress {
	return &zipCompress{
		dst: zip.NewWriter(file),
	}
}

type zipCompress struct {
	compress

	wg sync.WaitGroup

	dst *zip.Writer
}

func (zc *zipCompress) Close() error {
	zc.wg.Wait()
	return zc.dst.Close()
}

func (zc *zipCompress) NewFile(information formatInformation) (io.WriteCloser, error) {
	rp, wp := io.Pipe()
	go func() {
		zc.wg.Add(1)
		defer zc.wg.Done()

		var buf bytes.Buffer
		io.Copy(&buf, rp)

		header := &zip.FileHeader{
			Name:               filepath.Join(fmt.Sprintf("S%2d", information.SeasonNumber), information.Title),
			Modified:           time.Now(),
			Method:             zip.Deflate,
			UncompressedSize64: uint64(buf.Len()),
		}
		header.SetMode(0644)

		hw, _ := zc.dst.CreateHeader(header)
		io.Copy(hw, &buf)
	}()

	return wp, nil
}
