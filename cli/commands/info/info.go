package info

import (
	"fmt"
	"github.com/ByteDream/crunchy-cli/cli/commands"
	"github.com/ByteDream/crunchy-cli/utils"
	crunchyUtils "github.com/crunchy-labs/crunchyroll-go/v3/utils"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the logged in user",
	Args:  cobra.MinimumNArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := commands.LoadCrunchy(); err != nil {
			return err
		}

		return info()
	},
}

func info() error {
	account, err := utils.Crunchy.Account()
	if err != nil {
		return err
	}

	fmt.Println("Username:          ", account.Username)
	fmt.Println("Email:             ", account.Email)
	fmt.Println("Premium:           ", utils.Crunchy.Config.Premium)
	fmt.Println("Interface language:", crunchyUtils.LocaleLanguage(account.PreferredCommunicationLanguage))
	fmt.Println("Subtitle language: ", crunchyUtils.LocaleLanguage(account.PreferredContentSubtitleLanguage))
	fmt.Println("Created:           ", account.Created)
	fmt.Println("Account ID:        ", account.AccountID)

	return nil
}
