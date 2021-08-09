package cmd

import (
	"github.com/ByteDream/crunchyroll"
	"github.com/spf13/cobra"
	"io/ioutil"
)

var (
	sessionIDFlag bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to crunchyroll",
	Args:  cobra.RangeArgs(1, 2),

	RunE: func(cmd *cobra.Command, args []string) error {
		if sessionIDFlag {
			return loginSessionID(args[0], false)
		} else {
			return loginCredentials(args[0], args[1])
		}
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().BoolVar(&sessionIDFlag, "session-id", false, "session id")
}

func loginCredentials(email, password string) error {
	out.Debugln("Logging in via credentials")
	session, err := crunchyroll.LoginWithCredentials(email, password, locale, client)
	if err != nil {
		return err
	}
	return loginSessionID(session.SessionID, true)
}

func loginSessionID(sessionID string, alreadyChecked bool) error {
	if !alreadyChecked {
		out.Debugln("Logging in via session id")
		if _, err := crunchyroll.LoginWithSessionID(sessionID, locale, client); err != nil {
			return err
		}
	}
	out.Infoln("Due to security reasons, you have to login again on the next reboot")
	return ioutil.WriteFile(sessionIDPath, []byte(sessionID), 0777)
}
