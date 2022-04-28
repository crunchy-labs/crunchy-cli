package cmd

import (
	"context"
	"fmt"
	"github.com/ByteDream/crunchyroll-go"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
)

var Version = "development"

var (
	client  *http.Client
	crunchy *crunchyroll.Crunchyroll
	out     = newLogger(false, true, true)

	quietFlag   bool
	verboseFlag bool

	proxyFlag string

	useragentFlag string
)

var rootCmd = &cobra.Command{
	Use:     "crunchyroll",
	Version: Version,
	Short:   "Download crunchyroll videos with ease. See the wiki for details about the cli and library: https://github.com/ByteDream/crunchyroll-go/wiki",

	SilenceErrors: true,
	SilenceUsage:  true,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if verboseFlag {
			out = newLogger(true, true, true)
		} else if quietFlag {
			out = newLogger(false, false, false)
		}

		out.Debug("Executing `%s` command with %d arg(s)", cmd.Name(), len(args))

		client, err = createOrDefaultClient(proxyFlag, useragentFlag)
		return
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Disable all output")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Adds debug messages to the normal output")

	rootCmd.PersistentFlags().StringVarP(&proxyFlag, "proxy", "p", "", "Proxy to use")

	rootCmd.PersistentFlags().StringVar(&useragentFlag, "useragent", fmt.Sprintf("crunchyroll-go/%s", Version), "Useragent to do all request with")
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
		if !strings.HasSuffix(err.Error(), context.Canceled.Error()) {
			out.Exit("An error occurred: %v", err)
		}
		os.Exit(1)
	}
}
