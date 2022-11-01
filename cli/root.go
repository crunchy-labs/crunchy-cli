package cli

import (
	"context"
	"fmt"
	"github.com/crunchy-labs/crunchy-cli/cli/commands"
	"github.com/crunchy-labs/crunchy-cli/cli/commands/archive"
	"github.com/crunchy-labs/crunchy-cli/cli/commands/download"
	"github.com/crunchy-labs/crunchy-cli/cli/commands/info"
	"github.com/crunchy-labs/crunchy-cli/cli/commands/login"
	"github.com/crunchy-labs/crunchy-cli/cli/commands/update"
	"github.com/crunchy-labs/crunchy-cli/utils"
	"github.com/crunchy-labs/crunchyroll-go/v3"
	crunchyUtils "github.com/crunchy-labs/crunchyroll-go/v3/utils"
	"github.com/spf13/cobra"
	"os"
	"runtime/debug"
	"strings"
)

var (
	quietFlag   bool
	verboseFlag bool

	proxyFlag string

	langFlag string

	useragentFlag string
)

var RootCmd = &cobra.Command{
	Use:     "crunchy-cli",
	Version: utils.Version,
	Short:   "Download crunchyroll videos with ease. See the wiki for details about the cli and library: https://github.com/crunchy-labs/crunchy-cli/wiki",

	SilenceErrors: true,
	SilenceUsage:  true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if verboseFlag {
			utils.Log = commands.NewLogger(true, true, true)
		} else if quietFlag {
			utils.Log = commands.NewLogger(false, false, false)
		}

		if langFlag != "" {
			if !crunchyUtils.ValidateLocale(crunchyroll.LOCALE(langFlag)) {
				return fmt.Errorf("'%s' is not a valid language. Choose from %s", langFlag, strings.Join(utils.LocalesAsStrings(), ", "))
			}

			os.Setenv("CRUNCHY_LANG", langFlag)
		}

		utils.Log.Debug("Executing `%s` command with %d arg(s)", cmd.Name(), len(args))

		utils.Client, err = utils.CreateOrDefaultClient(proxyFlag, useragentFlag)
		return
	},
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Disable all output")
	RootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Adds debug messages to the normal output")

	RootCmd.PersistentFlags().StringVarP(&proxyFlag, "proxy", "p", "", "Proxy to use")

	RootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", fmt.Sprintf("Set language to use. If not set, it's received from the system locale dynamically. Choose from: %s", strings.Join(utils.LocalesAsStrings(), ", ")))

	RootCmd.PersistentFlags().StringVar(&useragentFlag, "useragent", fmt.Sprintf("crunchy-cli/%s", utils.Version), "Useragent to do all request with")

	RootCmd.AddCommand(archive.Cmd)
	RootCmd.AddCommand(download.Cmd)
	RootCmd.AddCommand(info.Cmd)
	RootCmd.AddCommand(login.Cmd)
	RootCmd.AddCommand(update.Cmd)

	utils.Log = commands.NewLogger(false, true, true)
}

func Execute() {
	RootCmd.CompletionOptions.HiddenDefaultCmd = true
	defer func() {
		if r := recover(); r != nil {
			if utils.Log.IsDev() {
				utils.Log.Err("%v: %s", r, debug.Stack())
			} else {
				utils.Log.Err("Unexpected error: %v", r)
			}
			os.Exit(1)
		}
	}()
	if err := RootCmd.Execute(); err != nil {
		if !strings.HasSuffix(err.Error(), context.Canceled.Error()) {
			utils.Log.Err("An error occurred: %v", err)
		}
		os.Exit(1)
	}
}
