// Package config loads and merges .harvey.yaml files.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultImage is used when no `image:` is configured, so `harv` works with zero
// config. Red Hat's UBI is fully-qualified (resolves on every runtime), multi-arch,
// and free to pull without auth. Override it with `image:`, `--image`, or HARVEY_IMAGE.
const DefaultImage = "registry.access.redhat.com/ubi9/ubi:latest"

// Build holds the inputs for `harv init` / `harv recreate`.
type Build struct {
	Containerfile string   `yaml:"containerfile"`
	Context       string   `yaml:"context"`
	Args          []string `yaml:"args"`   // --build-arg KEY=VALUE
	Target        string   `yaml:"target"` // --target STAGE
}

// Sandbox is the locked-down profile for running untrusted code / AI agents. It
// inverts the dev default: only the workspace is mounted (never $HOME), and a
// dedicated env is injected rather than the project's.
type Sandbox struct {
	Workspace string   `yaml:"workspace"` // the only rw mount (default: $PWD)
	Network   string   `yaml:"network"`   // none (default) | a network name
	Env       []string `yaml:"env"`       // KEY=VALUE injected into the sandbox
	EnvFile   []string `yaml:"envFile"`   // --env-file paths
	Memory    string   `yaml:"memory"`    // --memory limit
	Cpus      string   `yaml:"cpus"`      // --cpus limit
}

// Config is the .harvey.yaml schema. Every field is optional; later files overlay
// earlier ones, and anything left unset falls back to a built-in default.
type Config struct {
	Image     string   `yaml:"image"`
	Runtime   string   `yaml:"runtime"` // auto | container | podman | docker
	Shell     string   `yaml:"shell"`
	Platform  string   `yaml:"platform"` // os/arch for run + build
	Workdir   string   `yaml:"workdir"`  // overrides the default -w $PWD
	User      string   `yaml:"user"`     // -u name|uid[:gid]
	Network   string   `yaml:"network"`  // --network
	Mounts    []string `yaml:"mounts"`
	Env       []string `yaml:"env"`
	EnvFile   []string `yaml:"envFile"`   // --env-file (repeatable)
	RunArgs   []string `yaml:"runArgs"`   // raw flags appended to `run`
	PostBuild []string `yaml:"postBuild"` // commands run after a build
	Build     Build    `yaml:"build"`
	Sandbox   Sandbox  `yaml:"sandbox"`
}

// Load reads config and applies HARVEY_IMAGE / HARVEY_RUNTIME overrides. If explicit
// is non-empty only that file is read (and it must exist). Otherwise the user file
// (~/.config/harvey/config.yaml) is read first and the nearest .harvey.yaml found by
// walking up from $PWD overlays it. Unknown keys are an error. Call Expand after any
// CLI-flag overrides to resolve ~ and $VARs.
func Load(explicit string) (*Config, error) {
	cfg := &Config{Shell: "/bin/sh", Runtime: "auto", Image: DefaultImage}

	var paths []string
	if explicit != "" {
		paths = []string{explicit}
	} else {
		home, _ := os.UserHomeDir()
		paths = append(paths, filepath.Join(home, ".config", "harvey", "config.yaml"))
		if p := findProjectConfig(); p != "" {
			paths = append(paths, p)
		}
	}

	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) && explicit == "" {
				continue
			}
			return nil, err
		}
		// Decode into the existing struct: keys present in the file are set, absent
		// keys keep their current value. KnownFields makes a typo'd key an error.
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		err = dec.Decode(cfg)
		_ = f.Close()
		if err != nil && err != io.EOF { // EOF = empty file, which is fine
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}
	}

	if v := os.Getenv("HARVEY_IMAGE"); v != "" {
		cfg.Image = v
	}
	if v := os.Getenv("HARVEY_RUNTIME"); v != "" {
		cfg.Runtime = v
	}
	return cfg, nil
}

// findProjectConfig walks up from the working directory looking for the nearest
// .harvey.yaml, so `harv` works from any subdirectory of a project.
func findProjectConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, ".harvey.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached the filesystem root
			return ""
		}
		dir = parent
	}
}

// Expand resolves a leading ~ and any $VARs in every path-like field.
func (c *Config) Expand() {
	c.Image = expand(c.Image)
	c.Shell = expand(c.Shell)
	c.Workdir = expand(c.Workdir)
	c.Build.Containerfile = expand(c.Build.Containerfile)
	c.Build.Context = expand(c.Build.Context)
	for i := range c.Mounts {
		c.Mounts[i] = expand(c.Mounts[i])
	}
	for i := range c.Env {
		c.Env[i] = expand(c.Env[i])
	}
	for i := range c.EnvFile {
		c.EnvFile[i] = expand(c.EnvFile[i])
	}
	c.Sandbox.Workspace = expand(c.Sandbox.Workspace)
	for i := range c.Sandbox.Env {
		c.Sandbox.Env[i] = expand(c.Sandbox.Env[i])
	}
	for i := range c.Sandbox.EnvFile {
		c.Sandbox.EnvFile[i] = expand(c.Sandbox.EnvFile[i])
	}
}

func expand(s string) string {
	if strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, s[2:])
		}
	}
	return os.ExpandEnv(s)
}
