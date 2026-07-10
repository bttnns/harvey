package cmd

import (
	"fmt"

	"github.com/bttnns/harvey/internal/config"
	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var (
	buildArgs    []string
	buildTarget  string
	buildNoCache bool
)

var initCmd = &cobra.Command{
	Use:        "init",
	SuggestFor: []string{"build", "create"},
	Short:      "Build the image if it is not present",
	Args:       cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, rt, err := resolveBuildable()
		if err != nil {
			return err
		}
		if runtime.ImageExists(rt, cfg.Image) {
			fmt.Printf("image %q already present\n", cfg.Image)
			return nil
		}
		return buildAndHook(cfg, rt)
	},
}

var recreateCmd = &cobra.Command{
	Use:        "recreate",
	SuggestFor: []string{"build", "rebuild"},
	Short:      "Force a full image rebuild",
	Args:       cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, rt, err := resolveBuildable()
		if err != nil {
			return err
		}
		if runtime.ImageExists(rt, cfg.Image) {
			_ = runtime.RunWait(rt, "image", "rm", cfg.Image)
		}
		return buildAndHook(cfg, rt)
	},
}

// buildAndHook merges the build flags into config, builds, then runs any postBuild
// commands as throwaway containers (so writes land in the mounted $HOME).
func buildAndHook(cfg *config.Config, rt runtime.Driver) error {
	cfg.Build.Args = append(cfg.Build.Args, buildArgs...)
	if buildTarget != "" {
		cfg.Build.Target = buildTarget
	}
	if err := runtime.Build(rt, cfg, buildNoCache); err != nil {
		return err
	}
	for _, c := range cfg.PostBuild {
		fmt.Printf("postBuild: %s\n", c)
		if err := runtime.RunWait(rt, runtime.BuildRunArgs(rt, cfg, runtime.RunOpts{Command: c})...); err != nil {
			return fmt.Errorf("postBuild %q failed: %w", c, err)
		}
	}
	return nil
}

// resolveBuildable is resolve plus the guarantee that the config can build an image.
func resolveBuildable() (*config.Config, runtime.Driver, error) {
	cfg, rt, err := resolve()
	if err != nil {
		return nil, nil, err
	}
	if cfg.Image == "" {
		return nil, nil, fmt.Errorf("no image set; add 'image:' to .harvey.yaml or pass --image")
	}
	if cfg.Build.Containerfile == "" || cfg.Build.Context == "" {
		return nil, nil, fmt.Errorf("build.containerfile and build.context must be set to build %q", cfg.Image)
	}
	return cfg, rt, nil
}

func init() {
	for _, c := range []*cobra.Command{initCmd, recreateCmd} {
		c.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "build-time variable KEY=VALUE (repeatable)")
		c.Flags().StringVar(&buildTarget, "target", "", "target build stage")
		c.Flags().BoolVar(&buildNoCache, "no-cache", false, "do not use the build cache")
	}
	rootCmd.AddCommand(initCmd, recreateCmd)
}
