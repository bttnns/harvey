package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

// keepAlive is the no-op main process for a persistent container: a portable idle
// loop. `sleep infinity` is not available in every base image (busybox/BSD sleep
// reject it), so we loop over a finite sleep instead, which every POSIX shell honors.
const keepAlive = "while :; do sleep 3600; done"

var enterCmd = &cobra.Command{
	Use:        "enter [NAME] [command...]",
	SuggestFor: []string{"shell", "attach"},
	Short:      "Open (creating if needed) a PERSISTENT, re-enterable dev container",
	Long: `enter is Toolbx-style "pet" mode: a named container that survives between
throwaway runs, so a slow one-off setup (a downloaded model, a warmed cache, a
half-finished experiment) sticks around until you throw it away.

  harv enter              enter the container named for this project dir (harvey-<dir>)
  harv enter mybox        enter (or create) a container named exactly "mybox"
  harv enter mybox ls -l  run a command in it instead of opening a shell

With no NAME the container is named after the current directory, sanitized to
container-name rules and prefixed "harvey-" (e.g. ~/Dev/foo -> harvey-foo). If the
container is not running it is started detached with the dev profile (your $HOME
mounted, same as a bare "harv"); then a login shell is exec'd into it.

This is a CACHE, not a home: reproducible state belongs in your Containerfile, and
"harv rm NAME" discards the container guilt-free.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := deriveName()
		var command string
		if len(args) > 0 {
			name = args[0]
			command = strings.Join(args[1:], " ")
		}

		cfg, rt, err := resolve()
		if err != nil {
			return err
		}
		if err := requireImage(cfg, rt); err != nil {
			return err
		}

		// `start` is the portable existence-and-ensure-running check: it is a no-op on a
		// running container and restarts a stopped one (persisting its filesystem), on
		// both Apple `container` and Podman. It fails only when the container does not
		// exist, which is our signal to create it. Output is discarded so the normal
		// "not found -> create" path is silent.
		if exec.Command(rt.Binary(), "start", name).Run() != nil {
			args := runtime.BuildRunArgs(rt, cfg, runtime.RunOpts{
				Detached: true,
				Name:     name,
				Command:  keepAlive,
			})
			if err := runtime.RunWait(rt, args...); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "harv: created persistent container %q (harv rm %s to discard)\n", name, name)
		}

		// Hand off to an interactive login shell (or the given command) in the container.
		return runtime.Hand(rt, runtime.ExecArgs(cfg, name, command))
	},
}

// deriveName builds the default container name from the current directory: its
// basename sanitized to container-name rules, prefixed "harvey-". The prefix
// guarantees a valid leading character, so sanitizeName only has to clean the body.
func deriveName() string {
	base := ""
	if cwd, err := os.Getwd(); err == nil {
		base = filepath.Base(cwd)
	}
	body := sanitizeName(base)
	if body == "" {
		body = "session" // unnamable dir (root, empty); still give a stable name
	}
	return "harvey-" + body
}

// sanitizeName maps a string to the characters container runtimes accept in a name
// ([A-Za-z0-9_.-]): every other rune becomes '-', runs of '-' collapse to one, and
// leading/trailing separators are trimmed. It does not add the "harvey-" prefix.
func sanitizeName(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
		if !ok {
			r = '-'
		}
		if r == '-' {
			if prevDash {
				continue // collapse consecutive dashes
			}
			prevDash = true
		} else {
			prevDash = false
		}
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "-._")
}

func init() {
	// Pass inner-command flags straight through (harv enter box go test -v).
	enterCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(enterCmd)
}
