package cmd

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go/v2"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

var (
	loginPersistentFlag bool

	loginSessionIDFlag bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to crunchyroll",
	Args:  cobra.RangeArgs(1, 2),

	RunE: func(cmd *cobra.Command, args []string) error {
		if loginSessionIDFlag {
			return loginSessionID(args[0])
		} else {
			return loginCredentials(args[0], args[1])
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
	c, err := crunchyroll.LoginWithCredentials(user, password, systemLocale(false), client)
	if err != nil {
		return err
	}

	if loginPersistentFlag {
		if configDir, err := os.UserConfigDir(); err != nil {
			return fmt.Errorf("could not save credentials persistent: %w", err)
		} else {
			os.MkdirAll(filepath.Join(configDir, "crunchyroll-go"), 0755)
			if err = os.WriteFile(filepath.Join(configDir, "crunchyroll-go", "crunchy"), []byte(fmt.Sprintf("%s\n%s", user, password)), 0600); err != nil {
				return err
			}
			out.Info("The login information will be stored permanently UNENCRYPTED on your drive (%s)", filepath.Join(configDir, "crunchyroll-go", "crunchy"))
		}
	}
	if err = os.WriteFile(filepath.Join(os.TempDir(), ".crunchy"), []byte(c.SessionID), 0600); err != nil {
		return err
	}

	if !loginPersistentFlag {
		out.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}

func loginSessionID(sessionID string) error {
	out.Debug("Logging in via session id")
	if _, err := crunchyroll.LoginWithSessionID(sessionID, systemLocale(false), client); err != nil {
		out.Err(err.Error())
		os.Exit(1)
	}

	var err error
	if loginPersistentFlag {
		if configDir, err := os.UserConfigDir(); err != nil {
			return fmt.Errorf("could not save credentials persistent: %w", err)
		} else {
			os.MkdirAll(filepath.Join(configDir, "crunchyroll-go"), 0755)
			if err = os.WriteFile(filepath.Join(configDir, "crunchyroll-go", "crunchy"), []byte(sessionID), 0600); err != nil {
				return err
			}
			out.Info("The login information will be stored permanently UNENCRYPTED on your drive (%s)", filepath.Join(configDir, "crunchyroll-go", "crunchy"))
		}
	}
	if err = os.WriteFile(filepath.Join(os.TempDir(), ".crunchy"), []byte(sessionID), 0600); err != nil {
		return err
	}

	if !loginPersistentFlag {
		out.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}
