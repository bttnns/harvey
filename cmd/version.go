package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// Version is the harvey version, overridable at build time with
// -ldflags "-X github.com/bttnns/harvey/cmd.Version=v1.2.3".
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the harvey version and the detected runtime",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("harvey %s\n", Version)
		_, rt, err := resolve()
		if err != nil {
			fmt.Println("runtime: none detected")
			return nil
		}
		line := rt.Name()
		if out, err := exec.Command(rt.Binary(), "--version").Output(); err == nil {
			line += ": " + strings.TrimSpace(string(out))
		}
		fmt.Printf("runtime: %s\n", line)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
