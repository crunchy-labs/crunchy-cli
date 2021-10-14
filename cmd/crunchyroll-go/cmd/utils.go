package cmd

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/ByteDream/crunchyroll-go/utils"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var sessionIDPath = filepath.Join(os.TempDir(), ".crunchy")

type progress struct {
	status  bool
	message string
}

type logger struct {
	DebugLog *log.Logger
	InfoLog  *log.Logger
	ErrLog   *log.Logger

	devView bool

	progressWG sync.Mutex
	progress   chan progress
}

func newLogger(debug, info, err bool, color bool) *logger {
	debugLog, infoLog, errLog := log.New(io.Discard, "=> ", 0), log.New(io.Discard, "=> ", 0), log.New(io.Discard, "=> ", 0)

	debugColor, infoColor, errColor := "", "", ""
	if color && runtime.GOOS != "windows" {
		debugColor, infoColor, errColor = "\033[95m", "\033[96m", "\033[31m"
	}

	if debug {
		debugLog.SetOutput(&loggerWriter{original: os.Stdout, color: debugColor})
	}
	if info {
		infoLog.SetOutput(&loggerWriter{original: os.Stdout, color: infoColor})
	}
	if err {
		errLog.SetOutput(&loggerWriter{original: os.Stdout, color: errColor})
	}

	if debug {
		debugLog = log.New(debugLog.Writer(), "[debug] ", 0)
		infoLog = log.New(infoLog.Writer(), "[info] ", 0)
		errLog = log.New(errLog.Writer(), "[err] ", 0)
	}

	return &logger{
		DebugLog: debugLog,
		InfoLog:  infoLog,
		ErrLog:   errLog,

		devView: debug,
	}
}

func (l *logger) Empty() {
	if !l.devView && l.InfoLog.Writer() != io.Discard {
		fmt.Println()
	}
}

func (l *logger) StartProgress(message string) {
	if l.devView {
		l.InfoLog.Println(message)
		return
	}
	l.progress = make(chan progress)

	go func() {
		states := []string{"-", "\\", "|", "/"}
		for i := 0; ; i++ {
			l.progressWG.Lock()
			select {
			case p := <-l.progress:
				// clearing the last line
				fmt.Printf("\r%s\r", strings.Repeat(" ", len(l.InfoLog.Prefix())+len(message)+2))
				if p.status {
					successTag := "✔"
					if runtime.GOOS == "windows" {
						successTag = "~"
					}
					l.InfoLog.Printf("%s %s", successTag, p.message)
				} else {
					errorTag := "✘"
					if runtime.GOOS == "windows" {
						errorTag = "!"
					}
					l.ErrLog.Printf("%s %s", errorTag, p.message)
				}
				l.progress = nil
				l.progressWG.Unlock()
				return
			default:
				if i%10 == 0 {
					fmt.Printf("\r%s%s %s", l.InfoLog.Prefix(), states[i/10%4], message)
				}
				time.Sleep(35 * time.Millisecond)
				l.progressWG.Unlock()
			}
		}
	}()
}

func (l *logger) StartProgressf(message string, a ...interface{}) {
	l.StartProgress(fmt.Sprintf(message, a...))
}

func (l *logger) EndProgress(successful bool, message string) {
	if l.devView {
		if successful {
			l.InfoLog.Print(message)
		} else {
			l.ErrLog.Print(message)
		}
		return
	} else if l.progress != nil {
		l.progress <- progress{
			status:  successful,
			message: message,
		}
	}
}

func (l *logger) EndProgressf(successful bool, message string, a ...interface{}) {
	l.EndProgress(successful, fmt.Sprintf(message, a...))
}

func (l *logger) Debugln(v ...interface{}) {
	l.print(0, v...)
}

func (l *logger) Debugf(message string, a ...interface{}) {
	l.print(0, fmt.Sprintf(message, a...))
}

func (l *logger) Infoln(v ...interface{}) {
	l.print(1, v...)
}

func (l *logger) Infof(message string, a ...interface{}) {
	l.print(1, fmt.Sprintf(message, a...))
}

func (l *logger) Errln(v ...interface{}) {
	l.print(2, v...)
}

func (l *logger) Errf(message string, a ...interface{}) {
	l.print(2, fmt.Sprintf(message, a...))
}

func (l *logger) Fatalln(v ...interface{}) {
	l.print(2, v...)
	os.Exit(1)
}

func (l *logger) Fatalf(message string, a ...interface{}) {
	l.print(2, fmt.Sprintf(message, a...))
	os.Exit(1)
}

func (l *logger) print(level int, v ...interface{}) {
	if l.progress != nil {
		l.progressWG.Lock()
		defer l.progressWG.Unlock()
		fmt.Print("\r")
	}

	switch level {
	case 0:
		l.DebugLog.Print(v...)
	case 1:
		l.InfoLog.Print(v...)
	case 2:
		l.ErrLog.Print(v...)
	}
}

type loggerWriter struct {
	io.Writer

	original io.Writer
	color    string
}

func (lw *loggerWriter) Write(p []byte) (n int, err error) {
	if lw.color != "" {
		p = append([]byte(lw.color), p...)
		p = append(p, []byte("\033[0m")...)
	}
	return lw.original.Write(p)
}

// systemLocale receives the system locale
// https://stackoverflow.com/questions/51829386/golang-get-system-language/51831590#51831590
func systemLocale() crunchyroll.LOCALE {
	if runtime.GOOS != "windows" {
		if lang, ok := os.LookupEnv("LANG"); ok {
			return localeToLOCALE(strings.ReplaceAll(strings.Split(lang, ".")[0], "_", "-"))
		}
	} else {
		cmd := exec.Command("powershell", "Get-Culture | select -exp Name")
		if output, err := cmd.Output(); err != nil {
			return localeToLOCALE(strings.Trim(string(output), "\r\n"))
		}
	}
	return localeToLOCALE("en-US")
}

func localeToLOCALE(locale string) crunchyroll.LOCALE {
	if l := crunchyroll.LOCALE(locale); utils.ValidateLocale(l) {
		return l
	} else {
		out.Errf("%s is not a supported locale, using %s as fallback\n", locale, crunchyroll.US)
		return crunchyroll.US
	}
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
		out.Infof("Using custom proxy %s\n", proxy)
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
	ext := path.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	j := 0
	for ; ; j++ {
		if _, stat := os.Stat(filename); stat != nil && !os.IsExist(stat) {
			break
		}
		filename = fmt.Sprintf("%s (%d)%s", base, j, ext)
	}
	return filename, j != 0
}

func loadSessionID() (string, error) {
	if _, stat := os.Stat(sessionIDPath); os.IsNotExist(stat) {
		out.Fatalf("To use this command, login first. Type `%s login -h` to get help\n", os.Args[0])
	}
	body, err := ioutil.ReadFile(sessionIDPath)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(body), "\n", ""), nil
}

func loadCrunchy() {
	out.StartProgress("Logging in")
	sessionID, err := loadSessionID()
	if err == nil {
		if crunchy, err = crunchyroll.LoginWithSessionID(sessionID, locale, client); err != nil {
			out.EndProgress(false, err.Error())
			os.Exit(1)
		}
	} else {
		out.EndProgress(false, err.Error())
		os.Exit(1)
	}
	out.EndProgress(true, "Logged in")
	out.Debugf("Logged in with session id %s\n", sessionID)
}

func hasFFmpeg() bool {
	cmd := exec.Command("ffmpeg", "-h")
	return cmd.Run() == nil
}

func terminalWidth() int {
	if runtime.GOOS != "windows" {
		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		out, err := cmd.Output()
		if err != nil {
			return 60
		}
		width, err := strconv.Atoi(strings.Split(strings.ReplaceAll(string(out), "\n", ""), " ")[1])
		if err != nil {
			return 60
		}
		return width
	}
	return 60
}
