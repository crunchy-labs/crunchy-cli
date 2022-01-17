package cmd

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/ByteDream/crunchyroll-go/utils"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var sessionIDPath = filepath.Join(os.TempDir(), ".crunchy")

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
