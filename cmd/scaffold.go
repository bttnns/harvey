package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var scaffoldForce bool

const scaffoldTemplate = `# harvey config. Full reference: docs/spec.md.
image: my-dev-image
runtime: auto            # auto | container (macOS) | podman (Linux)
shell: /bin/sh
# platform: linux/amd64  # os/arch for cross-arch emulation
mounts:
  - $HOME:$HOME          # bind-mount $HOME at the same path
env:
  - HOME=$HOME
# envFile:
#   - .env
build:                   # only needed for "harv init" / "harv recreate"
  containerfile: ./Containerfile
  context: .
# postBuild:
#   - echo set up
`

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Write a starter .harvey.yaml in the current directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		const path = ".harvey.yaml"
		if _, err := os.Stat(path); err == nil && !scaffoldForce {
			return fmt.Errorf("%s already exists; pass --force to overwrite", path)
		}
		if err := os.WriteFile(path, []byte(scaffoldTemplate), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
		return nil
	},
}

func init() {
	scaffoldCmd.Flags().BoolVar(&scaffoldForce, "force", false, "overwrite an existing .harvey.yaml")
	rootCmd.AddCommand(scaffoldCmd)
}
