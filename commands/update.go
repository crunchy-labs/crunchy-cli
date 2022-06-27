package commands

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var (
	updateInstallFlag bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check if updates are available",
	Args:  cobra.MaximumNArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		return update()
	},
}

func init() {
	updateCmd.Flags().BoolVarP(&updateInstallFlag,
		"install",
		"i",
		false,
		"If set and a new version is available, the new version gets installed")

	rootCmd.AddCommand(updateCmd)
}

func update() error {
	var release map[string]interface{}

	resp, err := client.Get("https://api.github.com/repos/ByteDream/crunchy-cli/releases/latest")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}
	releaseVersion := strings.TrimPrefix(release["tag_name"].(string), "v")

	if Version == "development" {
		out.Info("Development version, update service not available")
		return nil
	}

	latestRelease := strings.SplitN(releaseVersion, ".", 4)
	if len(latestRelease) != 3 {
		return fmt.Errorf("latest tag name (%s) is not parsable", releaseVersion)
	}

	internalVersion := strings.SplitN(Version, ".", 4)
	if len(internalVersion) != 3 {
		return fmt.Errorf("internal version (%s) is not parsable", Version)
	}

	out.Info("Installed version is %s", Version)

	var hasUpdate bool
	for i := 0; i < 3; i++ {
		if latestRelease[i] < internalVersion[i] {
			out.Info("Local version is newer than version in latest release (%s)", releaseVersion)
			return nil
		} else if latestRelease[i] > internalVersion[i] {
			hasUpdate = true
		}
	}

	if !hasUpdate {
		out.Info("Version is up-to-date")
		return nil
	}

	out.Info("A new version is available (%s): https://github.com/ByteDream/crunchy-cli/releases/tag/v%s", releaseVersion, releaseVersion)

	if updateInstallFlag {
		if runtime.GOARCH != "amd64" {
			return fmt.Errorf("invalid architecture found (%s), only amd64 is currently supported for automatic updating. "+
				"You have to update manually (https://github.com/ByteDream/crunchy-cli)", runtime.GOARCH)
		}

		var downloadFile string
		switch runtime.GOOS {
		case "linux":
			yayCommand := exec.Command("pacman -Q crunchy-cli")
			if yayCommand.Run() == nil && yayCommand.ProcessState.Success() {
				out.Info("crunchy-cli was probably installed via an Arch Linux AUR helper (like yay). Updating via this AUR helper is recommended")
				return nil
			}
			downloadFile = fmt.Sprintf("crunchy-v%s_linux", releaseVersion)
		case "darwin":
			downloadFile = fmt.Sprintf("crunchy-v%s_darwin", releaseVersion)
		case "windows":
			downloadFile = fmt.Sprintf("crunchy-v%s_windows.exe", releaseVersion)
		default:
			return fmt.Errorf("invalid operation system found (%s), only linux, windows and darwin / macos are currently supported. "+
				"You have to update manually (https://github.com/ByteDream/crunchy-cli", runtime.GOOS)
		}

		out.SetProgress("Updating executable %s", os.Args[0])

		perms, err := os.Stat(os.Args[0])
		if err != nil {
			return err
		}
		os.Remove(os.Args[0])
		executeFile, err := os.OpenFile(os.Args[0], os.O_CREATE|os.O_WRONLY, perms.Mode())
		if err != nil {
			return err
		}
		defer executeFile.Close()

		resp, err := client.Get(fmt.Sprintf("https://github.com/ByteDream/crunchy-cli/releases/download/v%s/%s", releaseVersion, downloadFile))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if _, err = io.Copy(executeFile, resp.Body); err != nil {
			return err
		}

		out.StopProgress("Updated executable %s", os.Args[0])
	}

	return nil
}
