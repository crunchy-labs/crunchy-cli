package cmd

import (
	"github.com/ByteDream/crunchyroll"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
)

var (
	client  *http.Client
	locale  crunchyroll.LOCALE
	crunchy *crunchyroll.Crunchyroll
	out     = newLogger(false, true, true, colorFlag)

	quietFlag   bool
	verboseFlag bool
	proxyFlag   string
	localeFlag  string
	colorFlag   bool
)

var rootCmd = &cobra.Command{
	Use:   "crunchyroll",
	Short: "Download crunchyroll videos with ease",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if verboseFlag {
			out = newLogger(true, true, true, colorFlag)
		} else if quietFlag {
			out = newLogger(false, false, false, false)
		}

		out.DebugLog.Printf("Executing `%s` command with %d arg(s)\n", cmd.Name(), len(args))

		locale = localeToLOCALE(localeFlag)

		client, err = createOrDefaultClient(proxyFlag)
		return
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Disable all output")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Adds debug messages to the normal output")
	rootCmd.PersistentFlags().StringVarP(&proxyFlag, "proxy", "p", "", "Proxy to use")
	rootCmd.PersistentFlags().StringVarP(&localeFlag, "locale", "l", systemLocale(), "The locale to use")
	rootCmd.PersistentFlags().BoolVar(&colorFlag, "color", false, "Colored output. Only available on not windows systems")
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	defer func() {
		if r := recover(); r != nil {
			out.Errln(r)
			// change color to red
			if colorFlag && runtime.GOOS != "windows" {
				out.ErrLog.SetOutput(&loggerWriter{original: out.ErrLog.Writer(), color: "\033[31m"})
			}
			out.Debugln(string(debug.Stack()))
			os.Exit(2)
		}
	}()
	if err := rootCmd.Execute(); err != nil {
		out.Fatalln(err)
	}
}
