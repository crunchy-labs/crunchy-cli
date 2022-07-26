package commands

import (
	"fmt"
	"github.com/crunchy-labs/crunchy-cli/utils"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type DownloadProgress struct {
	Prefix  string
	Message string

	Total   int
	Current int

	Dev   bool
	Quiet bool

	lock sync.Mutex
}

func (dp *DownloadProgress) Update() {
	dp.update("", false)
}

func (dp *DownloadProgress) UpdateMessage(msg string, permanent bool) {
	dp.update(msg, permanent)
}

func (dp *DownloadProgress) update(msg string, permanent bool) {
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

	// I don't really know why +2 is needed here but without it the Printf below would not print to the line end
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

func terminalWidth() int {
	if runtime.GOOS != "windows" {
		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		res, err := cmd.Output()
		if err != nil {
			return 60
		}
		// on alpine linux the command `stty size` does not respond the terminal size
		// but something like "stty: standard input". this may also apply to other systems
		splitOutput := strings.SplitN(strings.ReplaceAll(string(res), "\n", ""), " ", 2)
		if len(splitOutput) == 1 {
			return 60
		}
		width, err := strconv.Atoi(splitOutput[1])
		if err != nil {
			return 60
		}
		return width
	}
	return 60
}

func LoadCrunchy() error {
	var encryptionKey []byte

	if utils.IsTempSession() {
		encryptionKey = nil
	} else {
		if encrypted, err := utils.IsSavedSessionEncrypted(); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("to use this command, login first. Type `%s login -h` to get help", os.Args[0])
			}
			return err
		} else if encrypted {
			encryptionKey, err = ReadLineSilent()
			if err != nil {
				return fmt.Errorf("failed to read password")
			}
		}
	}

	var err error
	utils.Crunchy, err = utils.LoadSession(encryptionKey)
	return err
}
