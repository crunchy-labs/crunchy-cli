package cmd

import (
	"fmt"
	"github.com/ByteDream/crunchyroll-go/v3/utils"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the logged in user",
	Args:  cobra.MinimumNArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		loadCrunchy()

		return info()
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func info() error {
	account, err := crunchy.Account()
	if err != nil {
		return err
	}

	fmt.Println("Username:          ", account.Username)
	fmt.Println("Email:             ", account.Email)
	fmt.Println("Premium:           ", crunchy.Config.Premium)
	fmt.Println("Interface language:", utils.LocaleLanguage(account.PreferredCommunicationLanguage))
	fmt.Println("Subtitle language: ", utils.LocaleLanguage(account.PreferredContentSubtitleLanguage))
	fmt.Println("Created:           ", account.Created)
	fmt.Println("Account ID:        ", account.AccountID)

	return nil
}
