package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/bttnns/harvey/internal/config"
	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the runtime, image, config, and platform",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ok := true
		check := func(label string, pass bool, detail string) {
			mark := "ok"
			if !pass {
				mark, ok = "FAIL", false
			}
			fmt.Printf("[%4s] %s%s\n", mark, label, detail)
		}

		// Config loads and validates (strict decode catches unknown keys).
		cfg, err := config.Load(flagConfig)
		if err != nil {
			check("config", false, ": "+err.Error())
			return fmt.Errorf("doctor found problems")
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
		check("config", true, " parsed, no unknown keys")

		// Runtime detected, with its version.
		rt, err := runtime.Detect(cfg.Runtime)
		if err != nil {
			check("runtime", false, ": "+err.Error())
			return fmt.Errorf("doctor found problems")
		}
		detail := ": " + rt.Name()
		if out, err := exec.Command(rt.Binary(), "--version").Output(); err == nil {
			detail += " (" + strings.TrimSpace(string(out)) + ")"
		}
		check("runtime", true, detail)

		// Image present.
		switch {
		case cfg.Image == "":
			check("image", false, ": none set")
		case runtime.ImageExists(rt, cfg.Image):
			check("image", true, fmt.Sprintf(": %q present", cfg.Image))
		default:
			check("image", false, fmt.Sprintf(": %q missing (run \"harv init\")", cfg.Image))
		}

		// Platform.
		if cfg.Platform == "" {
			check("platform", true, ": native (none set)")
		} else {
			check("platform", true, ": "+cfg.Platform)
		}

		if !ok {
			return fmt.Errorf("doctor found problems")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
