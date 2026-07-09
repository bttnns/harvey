package cmd

import (
	"strings"

	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox [command...]",
	Short: "Run a command or AI agent in a locked-down sandbox container",
	Long: `Run untrusted code or an AI coding agent (e.g. claude, codex, gemini) in a
hardened container. Unlike the default, the sandbox INVERTS the mounts: it mounts only
the workspace (never $HOME), gives an ephemeral writable HOME, drops the network and
all Linux capabilities, makes the root filesystem read-only, and injects only the env
you list under sandbox.env (so host secrets and credentials never enter the container).

With no command it opens a sandboxed shell; otherwise it runs the command:

  harv sandbox                 sandboxed shell
  harv sandbox claude          run the Claude Code agent, contained
  harv sandbox npm test        run untrusted project scripts, contained`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := runtime.RunOpts{Sandbox: true}
		if len(args) == 0 {
			o.Interactive = true
		} else {
			o.Command = strings.Join(args, " ")
		}
		return launch(o)
	},
}

func init() {
	// Pass the agent's own flags through (harv sandbox claude --dangerously-...).
	sandboxCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(sandboxCmd)
}
