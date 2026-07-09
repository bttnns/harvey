package cmd

import (
	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var cpCmd = &cobra.Command{
	Use:   "cp SRC DST",
	Short: "Copy files between host and container (use NAME:path for the container side)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, rt, err := resolve()
		if err != nil {
			return err
		}
		return runtime.Hand(rt, append([]string{"cp"}, args...))
	},
}

func init() {
	rootCmd.AddCommand(cpCmd)
}
