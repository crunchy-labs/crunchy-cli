package cmd

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/ByteDream/crunchyroll-go/utils"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ahh i love windows :)))
	invalidWindowsChars    = []string{"<", ">", ":", "\"", "/", "|", "\\", "?", "*"}
	invalidNotWindowsChars = []string{"/"}
)

var urlFilter = regexp.MustCompile(`(S(\d+))?(E(\d+))?((-)(S(\d+))?(E(\d+))?)?(,|$)`)

// systemLocale receives the system locale
// https://stackoverflow.com/questions/51829386/golang-get-system-language/51831590#51831590
func systemLocale() crunchyroll.LOCALE {
	if runtime.GOOS != "windows" {
		if lang, ok := os.LookupEnv("LANG"); ok {
			prefix := strings.Split(lang, "_")[0]
			suffix := strings.Split(strings.Split(lang, ".")[0], "_")[1]
			l := crunchyroll.LOCALE(fmt.Sprintf("%s-%s", prefix, suffix))
			if !utils.ValidateLocale(l) {
				out.Err("%s is not a supported locale, using %s as fallback", l, crunchyroll.US)
				l = crunchyroll.US
			}
			return l
		}
	} else {
		cmd := exec.Command("powershell", "Get-Culture | select -exp Name")
		if output, err := cmd.Output(); err == nil {
			l := crunchyroll.LOCALE(strings.Trim(string(output), "\r\n"))
			if !utils.ValidateLocale(l) {
				out.Err("%s is not a supported locale, using %s as fallback", l, crunchyroll.US)
				l = crunchyroll.US
			}
			return l
		}
	}
	out.Err("Failed to get locale, using %s", crunchyroll.US)
	return crunchyroll.US
}

func allLocalesAsStrings() (locales []string) {
	for _, locale := range utils.AllLocales {
		locales = append(locales, string(locale))
	}
	return
}

func createOrDefaultClient(proxy string) (*http.Client, error) {
	if proxy == "" {
		return http.DefaultClient, nil
	} else {
		out.Info("Using custom proxy %s", proxy)
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		client := &http.Client{
			Transport: &http.Transport{
				DisableCompression: true,
				Proxy:              http.ProxyURL(proxyURL),
			},
			Timeout: 30 * time.Second,
		}
		return client, nil
	}
}

func freeFileName(filename string) (string, bool) {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	// checks if a .tar stands before the "actual" file ending
	if extraExt := filepath.Ext(base); extraExt == ".tar" {
		ext = extraExt + ext
		base = strings.TrimSuffix(base, extraExt)
	}
	j := 0
	for ; ; j++ {
		if _, stat := os.Stat(filename); stat != nil && !os.IsExist(stat) {
			break
		}
		filename = fmt.Sprintf("%s (%d)%s", base, j, ext)
	}
	return filename, j != 0
}

func loadCrunchy() {
	out.SetProgress("Logging in")

	files := []string{filepath.Join(os.TempDir(), ".crunchy")}

	if runtime.GOOS != "windows" {
		usr, _ := user.Current()
		files = append(files, filepath.Join(usr.HomeDir, ".config/crunchy"))
	}

	var body []byte
	var err error
	for _, file := range files {
		if _, err = os.Stat(file); os.IsNotExist(err) {
			continue
		}
		body, err = os.ReadFile(file)
		break
	}
	if body == nil {
		out.Err("To use this command, login first. Type `%s login -h` to get help", os.Args[0])
		os.Exit(1)
	} else if err != nil {
		out.Err("Failed to read login information: %v", err)
		os.Exit(1)
	}

	split := strings.SplitN(string(body), "\n", 2)
	if len(split) == 1 || split[1] == "" {
		if crunchy, err = crunchyroll.LoginWithSessionID(split[0], systemLocale(), client); err != nil {
			out.StopProgress(err.Error())
			os.Exit(1)
		}
		out.Debug("Logged in with session id %s. BLANK THIS LINE OUT IF YOU'RE ASKED TO POST THE DEBUG OUTPUT SOMEWHERE", split[0])
	} else {
		if crunchy, err = crunchyroll.LoginWithCredentials(split[0], split[1], systemLocale(), client); err != nil {
			out.StopProgress(err.Error())
			os.Exit(1)
		}
		out.Debug("Logged in with username '%s' and password '%s'. BLANK THIS LINE OUT IF YOU'RE ASKED TO POST THE DEBUG OUTPUT SOMEWHERE", split[0], split[1])
	}

	out.StopProgress("Logged in")
}

func hasFFmpeg() bool {
	return exec.Command("ffmpeg", "-h").Run() == nil
}

func terminalWidth() int {
	if runtime.GOOS != "windows" {
		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		res, err := cmd.Output()
		if err != nil {
			return 60
		}
		width, err := strconv.Atoi(strings.Split(strings.ReplaceAll(string(res), "\n", ""), " ")[1])
		if err != nil {
			return 60
		}
		return width
	}
	return 60
}

func generateFilename(name, directory string) string {
	if runtime.GOOS != "windows" {
		for _, char := range invalidNotWindowsChars {
			name = strings.ReplaceAll(name, char, "")
		}
		out.Debug("Replaced invalid characters (not windows)")
	} else {
		for _, char := range invalidWindowsChars {
			name = strings.ReplaceAll(name, char, "")
		}
		out.Debug("Replaced invalid characters (windows)")
	}

	if directory != "" {
		name = filepath.Join(directory, name)
	}

	filename, changed := freeFileName(name)
	if changed {
		out.Info("File %s already exists, changing name to %s", name, filename)
	}

	return filename
}

func extractEpisodes(url string, locales ...crunchyroll.LOCALE) ([][]*crunchyroll.Episode, error) {
	var matches [][]string

	lastOpen := strings.LastIndex(url, "[")
	if strings.HasSuffix(url, "]") && lastOpen != -1 && lastOpen < len(url) {
		matches = urlFilter.FindAllStringSubmatch(url[lastOpen+1:len(url)-1], -1)

		var all string
		for _, match := range matches {
			all += match[0]
		}
		if all != url[lastOpen+1:len(url)-1] {
			return nil, fmt.Errorf("invalid episode filter")
		}
		url = url[:lastOpen]
	}

	final := make([][]*crunchyroll.Episode, len(locales))
	episodes, err := crunchy.ExtractEpisodesFromUrl(url, locales...)
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes: %v", err)
	}

	if len(episodes) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	if matches != nil {
		for _, match := range matches {
			fromSeason, fromEpisode, toSeason, toEpisode := -1, -1, -1, -1
			if match[2] != "" {
				fromSeason, _ = strconv.Atoi(match[2])
			}
			if match[4] != "" {
				fromEpisode, _ = strconv.Atoi(match[4])
			}
			if match[8] != "" {
				toSeason, _ = strconv.Atoi(match[8])
			}
			if match[10] != "" {
				toEpisode, _ = strconv.Atoi(match[10])
			}

			if match[6] != "-" {
				toSeason = fromSeason
				toEpisode = fromEpisode
			}

			tmpEps := make([]*crunchyroll.Episode, 0)
			for _, episode := range episodes {
				if fromSeason != -1 && (episode.SeasonNumber < fromSeason || (fromEpisode != -1 && episode.EpisodeNumber < fromEpisode)) {
					continue
				} else if fromSeason == -1 && fromEpisode != -1 && episode.EpisodeNumber < fromEpisode {
					continue
				} else if toSeason != -1 && (episode.SeasonNumber > toSeason || (toEpisode != -1 && episode.EpisodeNumber > toEpisode)) {
					continue
				} else if toSeason == -1 && toEpisode != -1 && episode.EpisodeNumber > toEpisode {
					continue
				} else {
					tmpEps = append(tmpEps, episode)
				}
			}

			if len(tmpEps) == 0 {
				return nil, fmt.Errorf("no episodes are matching the given filter")
			}

			episodes = tmpEps
		}
	}

	localeSorted, err := utils.SortEpisodesByAudio(episodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio locale: %v", err)
	}
	for i, locale := range locales {
		final[i] = append(final[i], localeSorted[locale]...)
	}

	return final, nil
}

type formatInformation struct {
	// the format to download
	format *crunchyroll.Format

	// additional formats which are only used by archive.go
	additionalFormats []*crunchyroll.Format

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

func (fi formatInformation) Format(source string) string {
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
		source = strings.ReplaceAll(source, "{"+fields.Field(i).Tag.Get("json")+"}", valueAsString)
	}

	return source
}

type downloadProgress struct {
	Prefix  string
	Message string

	Total   int
	Current int

	Dev   bool
	Quiet bool

	lock sync.Mutex
}

func (dp *downloadProgress) Update() {
	dp.update("", false)
}

func (dp *downloadProgress) UpdateMessage(msg string, permanent bool) {
	dp.update(msg, permanent)
}

func (dp *downloadProgress) update(msg string, permanent bool) {
	if dp.Quiet {
		return
	}

	if dp.Current >= dp.Total {
		return
	}

	dp.lock.Lock()
	defer dp.lock.Unlock()
	dp.Current++

	if msg == "" {
		msg = dp.Message
	}
	if permanent {
		dp.Message = msg
	}

	if dp.Dev {
		fmt.Printf("%s%s\n", dp.Prefix, msg)
		return
	}

	percentage := float32(dp.Current) / float32(dp.Total) * 100

	pre := fmt.Sprintf("%s%s [", dp.Prefix, msg)
	post := fmt.Sprintf("]%4d%% %8d/%d", int(percentage), dp.Current, dp.Total)

	// i don't really know why +2 is needed here but without it the Printf below would not print to the line end
	progressWidth := terminalWidth() - len(pre) - len(post) + 2
	repeatCount := int(percentage / float32(100) * float32(progressWidth))
	// it can be lower than zero when the terminal is very tiny
	if repeatCount < 0 {
		repeatCount = 0
	}
	progressPercentage := strings.Repeat("=", repeatCount)
	if dp.Current != dp.Total {
		progressPercentage += ">"
	}

	fmt.Printf("\r%s%-"+fmt.Sprint(progressWidth)+"s%s", pre, progressPercentage, post)
}
