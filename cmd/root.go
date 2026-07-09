// Package cmd defines the harv command tree.
package cmd

import (
	"fmt"
	"os"
	"strings"

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
	Use:   "harvey",
	Short: "harvey: throwaway, $HOME-mounted dev containers (alias: harv)",
	Long: `harvey runs commands inside a container image defined by a .harvey.yaml file.
It is commonly aliased to "harv" (alias harv=harvey); examples below use that alias.

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
		return runExec(args)
	},
}

// Execute runs the command tree and is the single entry point from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "harvey:", err)
		os.Exit(1)
	}
}

func init() {
	// Stop flag parsing at the first positional so flags meant for the inner command
	// (harv go test -v) pass straight through instead of erroring as unknown to harv.
	rootCmd.Flags().SetInterspersed(false)
	rootCmd.Version = Version

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagImage, "image", "", "image to use (overrides config)")
	pf.StringVar(&flagRuntime, "runtime", "", "runtime: auto|container|podman|docker")
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
	if cfg.Image == "" {
		return fmt.Errorf("no image set; add 'image:' to .harvey.yaml or pass --image")
	}
	if runtime.ImageExists(rt, cfg.Image) {
		return nil
	}
	if cfg.Build.Containerfile != "" && cfg.Build.Context != "" {
		return fmt.Errorf("image %q not found; run \"harv init\" to build it", cfg.Image)
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
	if err := requireImage(cfg, rt); err != nil {
		return err
	}
	if o.Sandbox {
		if missing := runtime.SandboxUnsupported(rt, cfg.Sandbox.Network); len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "harvey: sandbox on %s cannot enforce: %s (relying on remaining controls)\n",
				rt.Name(), strings.Join(missing, ", "))
		}
	}
	return runtime.Hand(rt, runtime.BuildRunArgs(rt, cfg, o))
}

func runShell() error { return launch(runtime.RunOpts{Interactive: true}) }

func runExec(command []string) error {
	return launch(runtime.RunOpts{Command: strings.Join(command, " ")})
}
