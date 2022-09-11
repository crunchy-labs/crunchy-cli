package update

import (
	"encoding/json"
	"fmt"
	"github.com/crunchy-labs/crunchy-cli/utils"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

var (
	updateInstallFlag bool
)

var Cmd = &cobra.Command{
	Use:   "update",
	Short: "Check if updates are available",
	Args:  cobra.MaximumNArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		return update()
	},
}

func init() {
	Cmd.Flags().BoolVarP(&updateInstallFlag,
		"install",
		"i",
		false,
		"If set and a new version is available, the new version gets installed")
}

func update() error {
	var release map[string]interface{}

	resp, err := utils.Client.Get("https://api.github.com/repos/crunchy-labs/crunchy-cli/releases/latest")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}
	releaseVersion := strings.TrimPrefix(release["tag_name"].(string), "v")

	if utils.Version == "development" {
		utils.Log.Info("Development version, update service not available")
		return nil
	}

	latestRelease := strings.SplitN(releaseVersion, ".", 4)
	if len(latestRelease) != 3 {
		return fmt.Errorf("latest tag name (%s) is not parsable", releaseVersion)
	}

	internalVersion := strings.SplitN(utils.Version, ".", 4)
	if len(internalVersion) != 3 {
		return fmt.Errorf("internal version (%s) is not parsable", utils.Version)
	}

	utils.Log.Info("Installed version is %s", utils.Version)

	var hasUpdate bool
	for i := 0; i < 3; i++ {
		if latestRelease[i] < internalVersion[i] {
			utils.Log.Info("Local version is newer than version in latest release (%s)", releaseVersion)
			return nil
		} else if latestRelease[i] > internalVersion[i] {
			hasUpdate = true
		}
	}

	if !hasUpdate {
		utils.Log.Info("Version is up-to-date")
		return nil
	}

	utils.Log.Info("A new version is available (%s): https://github.com/crunchy-labs/crunchy-cli/releases/tag/v%s", releaseVersion, releaseVersion)

	if updateInstallFlag {
		if runtime.GOARCH != "amd64" {
			return fmt.Errorf("invalid architecture found (%s), only amd64 is currently supported for automatic updating. "+
				"You have to update manually (https://github.com/crunchy-labs/crunchy-cli)", runtime.GOARCH)
		}

		var downloadFile string
		switch runtime.GOOS {
		case "linux":
			pacmanCommand := exec.Command("pacman -Q crunchy-cli")
			if pacmanCommand.Run() == nil && pacmanCommand.ProcessState.Success() {
				utils.Log.Info("crunchy-cli was probably installed via an Arch Linux AUR helper (like yay). Updating via this AUR helper is recommended")
				return nil
			}
			downloadFile = fmt.Sprintf("crunchy-v%s_linux", releaseVersion)
		case "darwin":
			downloadFile = fmt.Sprintf("crunchy-v%s_darwin", releaseVersion)
		case "windows":
			downloadFile = fmt.Sprintf("crunchy-v%s_windows.exe", releaseVersion)
		default:
			return fmt.Errorf("invalid operation system found (%s), only linux, windows and darwin / macos are currently supported. "+
				"You have to update manually (https://github.com/crunchy-labs/crunchy-cli", runtime.GOOS)
		}

		executePath := os.Args[0]
		var perms os.FileInfo
		// check if the path is relative, absolute or non (if so, the executable must be in PATH)
		if strings.HasPrefix(executePath, "."+string(os.PathSeparator)) || path.IsAbs(executePath) {
			if perms, err = os.Stat(os.Args[0]); err != nil {
				return err
			}
		} else {
			executePath, err = exec.LookPath(os.Args[0])
			if err != nil {
				return err
			}
			if perms, err = os.Stat(executePath); err != nil {
				return err
			}
		}

		utils.Log.SetProcess("Updating executable %s", executePath)

		if err = os.Remove(executePath); err != nil {
			return err
		}
		executeFile, err := os.OpenFile(executePath, os.O_CREATE|os.O_WRONLY, perms.Mode())
		if err != nil {
			return err
		}
		defer executeFile.Close()

		resp, err := utils.Client.Get(fmt.Sprintf("https://github.com/crunchy-labs/crunchy-cli/releases/download/v%s/%s", releaseVersion, downloadFile))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if _, err = io.Copy(executeFile, resp.Body); err != nil {
			return err
		}

		utils.Log.StopProcess("Updated executable %s", executePath)
	}

	return nil
}
