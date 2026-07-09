package cmd

import (
	"strings"

	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec NAME [command...]",
	Short: "Run a command (or open a shell) in a RUNNING container",
	Long: `Exec into an already-running container (e.g. one started with "harv serve"),
like "docker exec". With no command it opens an interactive login shell.

The throwaway one-off form is the bare "harv <cmd...>", not this.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, rt, err := resolve()
		if err != nil {
			return err
		}
		name, command := args[0], strings.Join(args[1:], " ")
		return runtime.Hand(rt, runtime.ExecArgs(cfg, name, command))
	},
}

func init() {
	// Pass inner-command flags through (harv exec app go test -v).
	execCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(execCmd)
}
