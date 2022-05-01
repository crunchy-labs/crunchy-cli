package cmd

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go/v2"
	"github.com/spf13/cobra"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

var (
	loginPersistentFlag bool

	loginSessionIDFlag bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to crunchyroll",
	Args:  cobra.RangeArgs(1, 2),

	Run: func(cmd *cobra.Command, args []string) {
		if loginSessionIDFlag {
			loginSessionID(args[0])
		} else {
			loginCredentials(args[0], args[1])
		}
	},
}

func init() {
	loginCmd.Flags().BoolVar(&loginPersistentFlag,
		"persistent",
		false,
		"If the given credential should be stored persistent")

	loginCmd.Flags().BoolVar(&loginSessionIDFlag,
		"session-id",
		false,
		"Use a session id to login instead of username and password")

	rootCmd.AddCommand(loginCmd)
}

func loginCredentials(user, password string) error {
	out.Debug("Logging in via credentials")
	if _, err := crunchyroll.LoginWithCredentials(user, password, systemLocale(false), client); err != nil {
		out.Err(err.Error())
		os.Exit(1)
	}

	return os.WriteFile(loginStorePath(), []byte(fmt.Sprintf("%s\n%s", user, password)), 0600)
}

func loginSessionID(sessionID string) error {
	out.Debug("Logging in via session id")
	if _, err := crunchyroll.LoginWithSessionID(sessionID, systemLocale(false), client); err != nil {
		out.Err(err.Error())
		os.Exit(1)
	}

	return os.WriteFile(loginStorePath(), []byte(sessionID), 0600)
}

func loginStorePath() string {
	path := filepath.Join(os.TempDir(), ".crunchy")
	if loginPersistentFlag {
		if runtime.GOOS != "windows" {
			usr, _ := user.Current()
			path = filepath.Join(usr.HomeDir, ".config/crunchy")
		}

		out.Info("The login information will be stored permanently UNENCRYPTED on your drive (%s)", path)
	} else if runtime.GOOS != "windows" {
		out.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return path
}
