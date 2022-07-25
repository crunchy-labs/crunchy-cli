package login

import (
	"bytes"
	"fmt"
	"github.com/ByteDream/crunchy-cli/cli/commands"
	"github.com/ByteDream/crunchy-cli/utils"
	"github.com/crunchy-labs/crunchyroll-go/v3"
	"github.com/spf13/cobra"
	"os"
)

var (
	loginPersistentFlag bool
	loginEncryptFlag    bool

	loginSessionIDFlag bool
	loginEtpRtFlag     bool
)

var Cmd = &cobra.Command{
	Use:   "login",
	Short: "Login to crunchyroll",
	Args:  cobra.RangeArgs(1, 2),

	RunE: func(cmd *cobra.Command, args []string) error {
		if loginSessionIDFlag {
			return loginSessionID(args[0])
		} else if loginEtpRtFlag {
			return loginEtpRt(args[0])
		} else {
			return loginCredentials(args[0], args[1])
		}
	},
}

func init() {
	Cmd.Flags().BoolVar(&loginPersistentFlag,
		"persistent",
		false,
		"If the given credential should be stored persistent")
	Cmd.Flags().BoolVar(&loginEncryptFlag,
		"encrypt",
		false,
		"Encrypt the given credentials (won't do anything if --session-id is given or --persistent is not given)")

	Cmd.Flags().BoolVar(&loginSessionIDFlag,
		"session-id",
		false,
		"Use a session id to login instead of username and password")
	Cmd.Flags().BoolVar(&loginEtpRtFlag,
		"etp-rt",
		false,
		"Use a etp rt cookie to login instead of username and password")

	Cmd.MarkFlagsMutuallyExclusive("session-id", "etp-rt")
}

func loginCredentials(user, password string) error {
	utils.Log.Debug("Logging in via credentials")
	c, err := crunchyroll.LoginWithCredentials(user, password, utils.SystemLocale(false), utils.Client)
	if err != nil {
		return err
	}

	if loginPersistentFlag {
		var passwd []byte
		if loginEncryptFlag {
			for {
				fmt.Print("Enter password: ")
				passwd, err = commands.ReadLineSilent()
				if err != nil {
					return err
				}
				fmt.Println()

				fmt.Print("Enter password again: ")
				repasswd, err := commands.ReadLineSilent()
				if err != nil {
					return err
				}
				fmt.Println()

				if bytes.Equal(passwd, repasswd) {
					break
				}
				fmt.Println("Passwords does not match, try again")
			}
		}
		if err = utils.SaveCredentialsPersistent(user, password, passwd); err != nil {
			return err
		}
		if !loginEncryptFlag {
			utils.Log.Warn("The login information will be stored permanently UNENCRYPTED on your drive. " +
				"To encrypt it, use the `--encrypt` flag")
		}
	}
	if err = utils.SaveSession(c); err != nil {
		return err
	}

	if !loginPersistentFlag {
		utils.Log.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}

func loginSessionID(sessionID string) error {
	utils.Log.Debug("Logging in via session id")
	var c *crunchyroll.Crunchyroll
	var err error
	if c, err = crunchyroll.LoginWithSessionID(sessionID, utils.SystemLocale(false), utils.Client); err != nil {
		return err
	}

	if loginPersistentFlag {
		if err = utils.SaveSessionPersistent(c); err != nil {
			return err
		}
		utils.Log.Warn("The login information will be stored permanently UNENCRYPTED on your drive")
	}
	if err = utils.SaveSession(c); err != nil {
		return err
	}

	if !loginPersistentFlag {
		utils.Log.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}

func loginEtpRt(etpRt string) error {
	utils.Log.Debug("Logging in via etp rt")
	var c *crunchyroll.Crunchyroll
	var err error
	if c, err = crunchyroll.LoginWithEtpRt(etpRt, utils.SystemLocale(false), utils.Client); err != nil {
		utils.Log.Err(err.Error())
		os.Exit(1)
	}

	if loginPersistentFlag {
		if err = utils.SaveSessionPersistent(c); err != nil {
			return err
		}
		utils.Log.Warn("The login information will be stored permanently UNENCRYPTED on your drive")
	}
	if err = utils.SaveSession(c); err != nil {
		return err
	}

	if !loginPersistentFlag {
		utils.Log.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}
