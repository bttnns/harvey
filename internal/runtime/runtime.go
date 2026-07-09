// Package runtime detects the container runtime and assembles/executes its commands.
//
// Runtimes are pluggable. Each one is a Driver (see container.go, podman.go,
// docker.go). To add support for another tool, implement Driver in a new file and
// add it to the drivers slice below; nothing else in the codebase changes.
package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/bttnns/harvey/internal/config"
)

// Driver describes how to drive one container runtime. Embed base for the shared
// defaults and override only the methods that differ for your runtime.
type Driver interface {
	Name() string           // identifier used in the config `runtime:` field
	Binary() string         // executable looked up on PATH
	ListArgs() []string     // subcommand that lists running containers
	ExtraRunOpts() []string // extra flags appended to every `run`
	// SandboxArgs returns the lockdown `run` flags this runtime supports for the
	// given network mode, plus a list of requested controls it can NOT provide
	// (so the caller can warn). Workspace mount, env, and resource limits are
	// runtime-independent and handled by the caller, not here.
	SandboxArgs(network string) (args, unsupported []string)
}

// drivers is also the auto-detection priority order. To add a runtime: implement
// Driver in its own file, then add one entry here.
var drivers = []Driver{
	appleContainer{base{name: "container", binary: "container"}},
	podman{base{name: "podman", binary: "podman"}},
	docker{base{name: "docker", binary: "docker"}},
}

// base supplies the Driver defaults most runtimes share: list with `ps`, no extra
// run opts. Concrete drivers embed it and override what differs.
type base struct {
	name   string
	binary string
}

func (b base) Name() string           { return b.name }
func (b base) Binary() string         { return b.binary }
func (b base) ListArgs() []string     { return []string{"ps"} }
func (b base) ExtraRunOpts() []string { return nil }

// SandboxArgs for the OCI runtimes (Docker/Podman): the full lockdown set. Apple
// `container` overrides this to drop the controls it lacks.
func (b base) SandboxArgs(network string) (args, unsupported []string) {
	if network == "" {
		network = "none"
	}
	return []string{
		"--read-only",
		"--tmpfs", "/tmp",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--network", network,
	}, nil
}

// Detect resolves the driver to use. A non-empty, non-"auto" preference must name a
// known runtime whose binary is on PATH; otherwise the first available driver in
// priority order wins.
func Detect(pref string) (Driver, error) {
	if pref != "" && pref != "auto" {
		d := find(pref)
		if d == nil {
			return nil, fmt.Errorf("unknown runtime %q (known: %s)", pref, known())
		}
		if _, err := exec.LookPath(d.Binary()); err != nil {
			return nil, fmt.Errorf("runtime %q (%s) not found on PATH", pref, d.Binary())
		}
		return d, nil
	}
	for _, d := range drivers {
		if _, err := exec.LookPath(d.Binary()); err == nil {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no container runtime found (tried %s)", known())
}

func find(name string) Driver {
	for _, d := range drivers {
		if d.Name() == name {
			return d
		}
	}
	return nil
}

func known() string {
	names := make([]string, len(drivers))
	for i, d := range drivers {
		names[i] = d.Name()
	}
	return strings.Join(names, ", ")
}

// ImageExists reports whether the image is present locally.
func ImageExists(d Driver, image string) bool {
	return exec.Command(d.Binary(), "image", "inspect", image).Run() == nil
}

// RunOpts selects the run mode and per-invocation values; Config supplies the rest.
type RunOpts struct {
	Interactive bool     // -it --rm, login shell
	Detached    bool     // -d --name
	Sandbox     bool     // locked-down profile (see sandboxArgs)
	Name        string   // container name (Detached)
	Ports       []string // already-normalized HOST:CONTAINER mappings
	Command     string   // shell command; empty means an interactive login shell
}

// BuildRunArgs assembles the full `run` argument slice from small reusable pieces:
// mode flags, platform/ports, then either the dev mounts or the sandbox lockdown,
// the driver's extra opts, the raw runArgs escape hatch, the image, and the shell.
func BuildRunArgs(d Driver, cfg *config.Config, o RunOpts) []string {
	args := []string{"run"}
	args = append(args, modeArgs(o)...)
	if cfg.Platform != "" {
		args = append(args, "--platform", cfg.Platform)
	}
	for _, p := range o.Ports {
		args = append(args, "-p", p)
	}
	if o.Sandbox {
		args = append(args, sandboxArgs(d, cfg)...)
	} else {
		args = append(args, devArgs(cfg)...)
	}
	args = append(args, d.ExtraRunOpts()...)
	args = append(args, cfg.RunArgs...)
	args = append(args, cfg.Image)
	args = append(args, shellArgs(cfg, o)...)
	return args
}

// modeArgs are the run-mode flags: detached+named, interactive throwaway, or a
// command throwaway. A command keeps stdin open (-i) so it can read piped input
// or drive an interactive agent (claude, codex), and gets a TTY (-t) only when
// both ends are real terminals, so a pseudo-tty's control bytes never leak into a
// pipe or a file.
func modeArgs(o RunOpts) []string {
	switch {
	case o.Detached:
		return []string{"-d", "--name", o.Name}
	case o.Interactive:
		return []string{"-it", "--rm"}
	case isTerminal(os.Stdin) && isTerminal(os.Stdout):
		return []string{"-it", "--rm"}
	default:
		return []string{"-i", "--rm"}
	}
}

// isTerminal reports whether f is attached to a character device (a TTY) rather
// than a pipe or regular file. Avoids pulling in golang.org/x/term for one check.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// devArgs are the default (trusted) mounts: configured env/mounts at their real
// paths, the working directory, user, and network.
func devArgs(cfg *config.Config) []string {
	var a []string
	for _, e := range cfg.Env {
		a = append(a, "-e", e)
	}
	for _, f := range cfg.EnvFile {
		a = append(a, "--env-file", f)
	}
	for _, m := range cfg.Mounts {
		a = append(a, "-v", m)
	}
	if wd := workdir(cfg); wd != "" {
		a = append(a, "-w", wd)
	}
	if cfg.User != "" {
		a = append(a, "-u", cfg.User)
	}
	if cfg.Network != "" {
		a = append(a, "--network", cfg.Network)
	}
	return a
}

// sandboxArgs INVERTS the dev default for running untrusted code / AI agents: it
// mounts ONLY the workspace (never $HOME or the configured mounts), gives an
// ephemeral writable HOME, injects only the explicitly listed sandbox env, applies
// resource limits, and adds the driver's lockdown flags (read-only rootfs, tmpfs,
// cap-drop, network isolation).
func sandboxArgs(d Driver, cfg *config.Config) []string {
	ws := cfg.Sandbox.Workspace
	if ws == "" {
		if cwd, err := os.Getwd(); err == nil {
			ws = cwd
		}
	}
	var a []string
	if ws != "" {
		a = append(a, "-v", ws+":"+ws, "-w", ws)
	}
	a = append(a, "-e", "HOME=/tmp") // writable, ephemeral (tmpfs below)
	for _, e := range cfg.Sandbox.Env {
		a = append(a, "-e", e)
	}
	for _, f := range cfg.Sandbox.EnvFile {
		a = append(a, "--env-file", f)
	}
	if cfg.Sandbox.Memory != "" {
		a = append(a, "--memory", cfg.Sandbox.Memory)
	}
	if cfg.Sandbox.Cpus != "" {
		a = append(a, "--cpus", cfg.Sandbox.Cpus)
	}
	if cfg.User != "" {
		a = append(a, "-u", cfg.User)
	}
	lock, _ := d.SandboxArgs(cfg.Sandbox.Network)
	return append(a, lock...)
}

// shellArgs is the trailing shell invocation: a login shell, or a command run
// through one.
func shellArgs(cfg *config.Config, o RunOpts) []string {
	if o.Command == "" {
		return []string{cfg.Shell, "-l"}
	}
	return []string{cfg.Shell, "-lc", o.Command}
}

// workdir is the configured working directory, or the current directory.
func workdir(cfg *config.Config) string {
	if cfg.Workdir != "" {
		return cfg.Workdir
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

// SandboxUnsupported lists the lockdown controls the runtime cannot provide for the
// given network mode, so a caller can warn the user about the reduced posture.
func SandboxUnsupported(d Driver, network string) []string {
	_, unsupported := d.SandboxArgs(network)
	return unsupported
}

// ExecArgs builds args to exec into a RUNNING container: an interactive login shell
// when command is empty, else the command run through the login shell.
func ExecArgs(cfg *config.Config, name, command string) []string {
	if command == "" {
		return []string{"exec", "-it", name, cfg.Shell, "-l"}
	}
	return []string{"exec", name, cfg.Shell, "-lc", command}
}

// Build builds the configured image from its Containerfile and context, honoring the
// platform, build args, and target from config plus a no-cache override.
func Build(d Driver, cfg *config.Config, noCache bool) error {
	args := []string{"build", "-t", cfg.Image, "-f", cfg.Build.Containerfile}
	if cfg.Platform != "" {
		args = append(args, "--platform", cfg.Platform)
	}
	for _, a := range cfg.Build.Args {
		args = append(args, "--build-arg", a)
	}
	if cfg.Build.Target != "" {
		args = append(args, "--target", cfg.Build.Target)
	}
	if noCache {
		args = append(args, "--no-cache")
	}
	args = append(args, cfg.Build.Context)
	return RunWait(d, args...)
}

// Hand replaces this process with the runtime command (like `exec` in a shell):
// stdio is inherited and the exit code propagates. Used for the final handoff.
func Hand(d Driver, args []string) error {
	bin, err := exec.LookPath(d.Binary())
	if err != nil {
		return err
	}
	return syscall.Exec(bin, append([]string{d.Binary()}, args...), os.Environ())
}

// RunWait runs the runtime command as a child and waits, so steps can be chained
// (recreate removes the image, then builds).
func RunWait(d Driver, args ...string) error {
	c := exec.Command(d.Binary(), args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}
