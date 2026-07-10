// Package cmd defines the harv command tree.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/fang/v2"
	"github.com/bttnns/harvey/internal/config"
	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

// Global flags, shared via resolve. They override the config file and env vars.
var (
	flagImage    string
	flagRuntime  string
	flagConfig   string
	flagPlatform string
)

var rootCmd = &cobra.Command{
	Use:   "harv",
	Short: "harv: throwaway, $HOME-mounted dev containers",
	Long: `harv runs commands inside a container image defined by a .harvey.yaml file.

With no arguments it opens an interactive login shell. Given a command it runs that
command in a throwaway container:

  harv                  interactive shell
  harv go test ./...    one-off command
  harv serve --name app -p 3000 npm run dev -- -H 0.0.0.0 -p 3000`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runShell()
		}
		// harv's bare form ships any command into the container, so a mistyped
		// management verb (harv stop) would otherwise be run inside the container and
		// fail confusingly. Catch the common ones and name the real command instead.
		if target, ok := verbAliases[args[0]]; ok {
			return usageErrorf(`unknown command %q; did you mean "harv %s"? (run "harv --help" for the command list)`, args[0], target)
		}
		return runExec(args)
	},
}

// verbAliases maps a mistaken management verb to the harv command that does the job,
// so a near-miss resolves to guidance instead of a confusing in-container failure.
// Kept in sync with the SuggestFor lists on the real commands (see their init funcs),
// which cover the standard cobra unknown-subcommand path.
var verbAliases = map[string]string{
	"stop":    "rm",
	"kill":    "rm",
	"remove":  "rm",
	"delete":  "rm",
	"build":   "recreate",
	"rebuild": "recreate",
	"shell":   "enter",
	"attach":  "enter",
	"list":    "ls",
}

// Execute runs the command tree and is the single entry point from main. It wraps the
// root command with fang for styled help/usage/errors, a build-info --version, a man
// page, and shell completions; fang's writers honor NO_COLOR / CLICOLOR / CLICOLOR_FORCE
// / TERM=dumb and never color a non-terminal. The exit code follows harv's contract
// (see docs/spec.md and exit.go).
func Execute() {
	err := fang.Execute(context.Background(), rootCmd, fang.WithVersion(Version))
	if err != nil {
		os.Exit(exitCode(err))
	}
}

func init() {
	// Stop flag parsing at the first positional so flags meant for the inner command
	// (harv go test -v) pass straight through instead of erroring as unknown to harv.
	rootCmd.Flags().SetInterspersed(false)
	rootCmd.Version = Version
	// Tag flag-parse errors (unknown flag, bad value) as usage errors so Execute maps
	// them to exit code 2 (see exit.go).
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err}
	})

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagImage, "image", "", "image to use (overrides config)")
	pf.StringVar(&flagRuntime, "runtime", "", "runtime: auto|container|podman")
	pf.StringVar(&flagConfig, "config", "", "path to a .harvey.yaml file")
	pf.StringVar(&flagPlatform, "platform", "", "platform os/arch, e.g. linux/amd64 (overrides config)")
}

// resolve loads config, applies flag overrides and expansion, and detects the runtime.
// It is the shared front half of every command.
func resolve() (*config.Config, runtime.Driver, error) {
	cfg, err := config.Load(flagConfig)
	if err != nil {
		return nil, nil, err
	}
	if flagImage != "" {
		cfg.Image = flagImage
	}
	if flagRuntime != "" {
		cfg.Runtime = flagRuntime
	}
	if flagPlatform != "" {
		cfg.Platform = flagPlatform
	}
	cfg.Expand()
	rt, err := runtime.Detect(cfg.Runtime)
	if err != nil {
		return nil, nil, err
	}
	return cfg, rt, nil
}

// requireImage fails unless a usable image is configured. A present image is fine.
// A missing image is only an error when the config builds it from a Containerfile
// (we never trigger a surprise build); a plain registry image with no build config
// is left for the runtime to pull on run, so "just point at an image and go" works
// with no Containerfile to author.
func requireImage(cfg *config.Config, rt runtime.Driver) error {
	return requireImageNamed(cfg, rt, cfg.Image)
}

// requireImageNamed is requireImage for an explicit image, so a sandbox run can verify
// the image it will actually use (cfg.SandboxImage(): the dedicated sandbox.image, else
// the top-level image) instead of the dev one.
func requireImageNamed(cfg *config.Config, rt runtime.Driver, image string) error {
	if image == "" {
		return fmt.Errorf("no image set; add 'image:' to .harvey.yaml or pass --image")
	}
	if runtime.ImageExists(rt, image) {
		return nil
	}
	if cfg.Build.Containerfile != "" && cfg.Build.Context != "" {
		return fmt.Errorf("image %q not found; run \"harv init\" to build it", image)
	}
	return nil // registry image; the runtime pulls it on first run
}

// launch is the shared run path for every command that starts a container (shell,
// exec-throwaway, serve, sandbox): resolve config + runtime, require the image, warn
// about any sandbox controls the runtime can't provide, then hand off the run.
func launch(o runtime.RunOpts) error {
	cfg, rt, err := resolve()
	if err != nil {
		return err
	}
	image := cfg.Image
	if o.Sandbox {
		image = cfg.SandboxImage() // dedicated sandbox.image, else falls back to cfg.Image
	}
	if err := requireImageNamed(cfg, rt, image); err != nil {
		return err
	}
	if o.Sandbox {
		if missing := runtime.SandboxUnsupported(rt, cfg.Sandbox.Network); len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "harv: sandbox on %s cannot enforce: %s (relying on remaining controls)\n",
				rt.Name(), strings.Join(missing, ", "))
		}
	}
	return runtime.Hand(rt, runtime.BuildRunArgs(rt, cfg, o))
}

func runShell() error { return launch(runtime.RunOpts{Interactive: true}) }

func runExec(command []string) error {
	return launch(runtime.RunOpts{Command: strings.Join(command, " ")})
}
