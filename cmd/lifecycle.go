package cmd

import (
	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var logsFollow bool

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"ps"}, // accept either spelling; translated to the runtime's verb
	Short:   "List running containers",
	// Let harv's own flags (e.g. --runtime) parse, but pass any runtime-native flags
	// (e.g. harv ls -a) straight through instead of erroring on them.
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE: func(cmd *cobra.Command, args []string) error {
		_, rt, err := resolve()
		if err != nil {
			return err
		}
		return runtime.Hand(rt, append(rt.ListArgs(), args...))
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs NAME",
	Short: "Show or follow a container's logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, rt, err := resolve()
		if err != nil {
			return err
		}
		out := []string{"logs"}
		if logsFollow {
			out = append(out, "-f")
		}
		return runtime.Hand(rt, append(out, args[0]))
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm NAME",
	Short: "Stop and remove a container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, rt, err := resolve()
		if err != nil {
			return err
		}
		return runtime.Hand(rt, []string{"rm", "-f", args[0]})
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "follow log output")
	rootCmd.AddCommand(lsCmd, logsCmd, rmCmd)
}
