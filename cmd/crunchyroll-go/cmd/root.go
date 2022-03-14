package cmd

import (
	"context"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"runtime/debug"
)

var (
	client  *http.Client
	crunchy *crunchyroll.Crunchyroll
	out     = newLogger(false, true, true)

	quietFlag   bool
	verboseFlag bool
	proxyFlag   string
)

var rootCmd = &cobra.Command{
	Use:   "crunchyroll",
	Short: "Download crunchyroll videos with ease",

	SilenceErrors: true,
	SilenceUsage:  true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if verboseFlag {
			out = newLogger(true, true, true)
		} else if quietFlag {
			out = newLogger(false, false, false)
		}

		out.DebugLog.Printf("Executing `%s` command with %d arg(s)\n", cmd.Name(), len(args))

		client, err = createOrDefaultClient(proxyFlag)
		return
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Disable all output")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Adds debug messages to the normal output")
	rootCmd.PersistentFlags().StringVarP(&proxyFlag, "proxy", "p", "", "Proxy to use")
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	defer func() {
		if r := recover(); r != nil {
			if out.IsDev() {
				out.Err("%v: %s", r, debug.Stack())
			} else {
				out.Err("Unexpected error: %v", r)
			}
			os.Exit(1)
		}
	}()
	if err := rootCmd.Execute(); err != nil {
		if err != context.Canceled {
			out.Exit("An error occurred: %v", err)
		}
		os.Exit(1)
	}
}
