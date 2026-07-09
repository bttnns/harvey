package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// fredCmd is a hidden tribute: Harvey, the dog this tool is named for, carried a
// stuffed rabbit named Fred everywhere, so harv carries him too.
var fredCmd = &cobra.Command{
	Use:    "fred",
	Short:  "Harvey's stuffed rabbit",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(`   (\(\
   ( -.-)   Harvey's stuffed rabbit.
   o_(")(")
`)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fredCmd)
}
