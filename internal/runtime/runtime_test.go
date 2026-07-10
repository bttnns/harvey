package runtime

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/bttnns/harvey/internal/config"
)

// testConfig has a dev mount, dev env, and a hostile runArgs escape hatch. The sandbox
// tests assert none of these leak into a locked-down run; the dev test asserts they do.
func testConfig() *config.Config {
	return &config.Config{
		Image:   "img",
		Shell:   "/bin/sh",
		Mounts:  []string{"/home/me:/home/me"},
		Env:     []string{"SECRET=1"},
		RunArgs: []string{"-v", "/evil:/evil"},
		Sandbox: config.Sandbox{
			Workspace: "/work",
			Network:   "none",
		},
	}
}

func joined(a []string) string { return strings.Join(a, " ") }

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// sandboxDrivers are the concrete drivers, built directly (constructors are unexported).
var sandboxDrivers = []Driver{
	appleContainer{base{name: "container", binary: "container"}},
	podman{base{name: "podman", binary: "podman"}},
}

// A sandbox run must apply the lockdown flags and must NOT inherit dev-mode extras: no
// SELinux disable, no runArgs escape hatch, no dev mounts. Runs with no runtime on PATH
// (BuildRunArgs is pure string assembly).
func TestSandboxDropsDevExtras(t *testing.T) {
	for _, d := range sandboxDrivers {
		d := d
		t.Run(d.Name(), func(t *testing.T) {
			cfg := testConfig()
			args := BuildRunArgs(d, cfg, RunOpts{Sandbox: true})
			s := joined(args)

			// Lockdown flags every driver supports.
			for _, want := range []string{"--read-only", "--tmpfs", "--cap-drop"} {
				if !contains(args, want) {
					t.Errorf("sandbox args missing lockdown flag %q: %v", want, args)
				}
			}
			// The workspace is the only bind mount, and it is the writable root.
			if !contains(args, "/work:/work") {
				t.Errorf("sandbox args missing workspace mount: %v", args)
			}
			// Must NOT weaken the fence.
			if strings.Contains(s, "label=disable") {
				t.Errorf("sandbox leaked SELinux disable: %v", args)
			}
			if strings.Contains(s, "/evil") {
				t.Errorf("sandbox leaked runArgs escape hatch: %v", args)
			}
			if strings.Contains(s, "/home/me:/home/me") {
				t.Errorf("sandbox leaked dev mount: %v", args)
			}
			if strings.Contains(s, "SECRET=1") {
				t.Errorf("sandbox leaked dev env: %v", args)
			}
		})
	}
}

// Podman-specific: no-new-privileges and --network none are enforced, and keep-id is
// present (so the container user can write the mounted workspace) WITHOUT label=disable.
func TestSandboxPodmanFlags(t *testing.T) {
	d := podman{base{name: "podman", binary: "podman"}}
	args := BuildRunArgs(d, testConfig(), RunOpts{Sandbox: true})
	for _, want := range []string{"no-new-privileges", "--network", "none", "--userns=keep-id"} {
		if !contains(args, want) {
			t.Errorf("podman sandbox missing %q: %v", want, args)
		}
	}
}

// Dev mode is unchanged: the dev mounts/env, the driver's ExtraRunOpts (including the
// Podman label=disable), and the runArgs escape hatch are all present, in order.
func TestDevKeepsExtrasAndRunArgs(t *testing.T) {
	d := podman{base{name: "podman", binary: "podman"}}
	args := BuildRunArgs(d, testConfig(), RunOpts{Command: "echo hi"})
	s := joined(args)

	for _, want := range []string{"/home/me:/home/me", "SECRET=1", "label=disable", "--userns=keep-id", "/evil:/evil"} {
		if !strings.Contains(s, want) {
			t.Errorf("dev args missing %q: %v", want, args)
		}
	}
	// Order: dev mounts, then ExtraRunOpts, then runArgs, then the image.
	iExtra := strings.Index(s, "label=disable")
	iRun := strings.Index(s, "/evil:/evil")
	iImg := strings.Index(s, " img ")
	if iExtra >= iRun || iRun >= iImg {
		t.Errorf("dev args out of order (extras < runArgs < image): %v", args)
	}
}

// count reports how many times s appears as an element of a.
func count(a []string, s string) int {
	n := 0
	for _, x := range a {
		if x == s {
			n++
		}
	}
	return n
}

// homeSpec is the $HOME:$HOME bind spec dev mode adds by default, or "" if $HOME is
// unresolvable (in which case the home-mount tests are skipped).
func homeSpec(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no resolvable $HOME")
	}
	return home + ":" + home
}

// Dev mode mounts $HOME at the same path by default (zero config), exactly once.
func TestDevMountsHomeByDefault(t *testing.T) {
	spec := homeSpec(t)
	d := podman{base{name: "podman", binary: "podman"}}
	cfg := &config.Config{Image: "img", Shell: "/bin/sh"}
	args := BuildRunArgs(d, cfg, RunOpts{Command: "echo hi"})
	if n := count(args, spec); n != 1 {
		t.Errorf("expected exactly one %q mount by default, got %d: %v", spec, n, args)
	}
}

// If the user also lists $HOME:$HOME in mounts, it is not added twice.
func TestDevHomeMountDeduped(t *testing.T) {
	spec := homeSpec(t)
	d := podman{base{name: "podman", binary: "podman"}}
	cfg := &config.Config{Image: "img", Shell: "/bin/sh", Mounts: []string{spec}}
	args := BuildRunArgs(d, cfg, RunOpts{Command: "echo hi"})
	if n := count(args, spec); n != 1 {
		t.Errorf("expected %q exactly once when user lists it too, got %d: %v", spec, n, args)
	}
}

// `home: false` opts out: no $HOME mount at all.
func TestDevHomeMountOptOut(t *testing.T) {
	spec := homeSpec(t)
	no := false
	d := podman{base{name: "podman", binary: "podman"}}
	cfg := &config.Config{Image: "img", Shell: "/bin/sh", Home: &no}
	args := BuildRunArgs(d, cfg, RunOpts{Command: "echo hi"})
	if n := count(args, spec); n != 0 {
		t.Errorf("home:false should omit the $HOME mount, got %d %q: %v", n, spec, args)
	}
}

// Sandbox mode NEVER mounts $HOME, regardless of the dev default.
func TestSandboxNeverMountsHome(t *testing.T) {
	spec := homeSpec(t)
	cfg := &config.Config{Image: "img", Shell: "/bin/sh", Sandbox: config.Sandbox{Workspace: "/work"}}
	for _, d := range sandboxDrivers {
		d := d
		t.Run(d.Name(), func(t *testing.T) {
			args := BuildRunArgs(d, cfg, RunOpts{Sandbox: true})
			if contains(args, spec) {
				t.Errorf("sandbox leaked $HOME mount %q: %v", spec, args)
			}
		})
	}
}

// trailingImage returns the image arg: the element just before the trailing shell
// invocation (cfg.Shell ... ), which BuildRunArgs appends right after the image.
func trailingImage(t *testing.T, args []string, shell string) string {
	t.Helper()
	for i, a := range args {
		if a == shell && i > 0 {
			return args[i-1]
		}
	}
	t.Fatalf("no shell %q found in args: %v", shell, args)
	return ""
}

// A sandbox run uses sandbox.image when set, and falls back to cfg.Image when it is not.
// Dev mode always uses cfg.Image and never the sandbox image.
func TestBuildRunArgsSandboxImage(t *testing.T) {
	d := podman{base{name: "podman", binary: "podman"}}

	// sandbox.image set: sandbox uses it, dev still uses cfg.Image.
	cfg := &config.Config{Image: "dev-img", Shell: "/bin/sh", Sandbox: config.Sandbox{Image: "sbx-img", Workspace: "/work"}}
	if got := trailingImage(t, BuildRunArgs(d, cfg, RunOpts{Sandbox: true}), cfg.Shell); got != "sbx-img" {
		t.Errorf("sandbox image = %q, want %q", got, "sbx-img")
	}
	if got := trailingImage(t, BuildRunArgs(d, cfg, RunOpts{Command: "echo hi"}), cfg.Shell); got != "dev-img" {
		t.Errorf("dev image = %q, want %q (sandbox.image must not leak into dev)", got, "dev-img")
	}

	// sandbox.image unset: sandbox falls back to cfg.Image.
	cfg2 := &config.Config{Image: "dev-img", Shell: "/bin/sh", Sandbox: config.Sandbox{Workspace: "/work"}}
	if got := trailingImage(t, BuildRunArgs(d, cfg2, RunOpts{Sandbox: true}), cfg2.Shell); got != "dev-img" {
		t.Errorf("sandbox image (unset) = %q, want fallback %q", got, "dev-img")
	}
}

// TestBuildRunArgsGolden pins the full `run` argument slice for the three run modes
// (dev, serve, sandbox) on both drivers. These are exact golden slices: any change to
// flag order or content is a deliberate, reviewable edit, not an accident. The modes
// with TTY-dependent mode flags are pinned via Interactive/Detached, and $HOME/workdir
// are set explicitly, so the slice is deterministic regardless of the test's stdio or
// working directory.
func TestBuildRunArgsGolden(t *testing.T) {
	no := false
	newCfg := func() *config.Config {
		return &config.Config{
			Image:   "img",
			Shell:   "/bin/sh",
			Home:    &no,  // opt out of the $HOME mount so the slice is deterministic
			Workdir: "/w", // explicit so it does not depend on os.Getwd
			Env:     []string{"E=1"},
			Mounts:  []string{"/m:/m"},
			RunArgs: []string{"--cap-add", "SYS_PTRACE"},
			Sandbox: config.Sandbox{Workspace: "/work", Network: "none"},
		}
	}
	apple := appleContainer{base{name: "container", binary: "container"}}
	pod := podman{base{name: "podman", binary: "podman"}}

	cases := []struct {
		name string
		d    Driver
		o    RunOpts
		want []string
	}{
		{
			"dev/podman", pod, RunOpts{Interactive: true},
			[]string{"run", "-it", "--rm", "-e", "E=1", "-v", "/m:/m", "-w", "/w",
				"--userns=keep-id", "--security-opt", "label=disable",
				"--cap-add", "SYS_PTRACE", "img", "/bin/sh", "-l"},
		},
		{
			"dev/apple", apple, RunOpts{Interactive: true},
			[]string{"run", "-it", "--rm", "-e", "E=1", "-v", "/m:/m", "-w", "/w",
				"--cap-add", "SYS_PTRACE", "img", "/bin/sh", "-l"},
		},
		{
			"serve/podman", pod,
			RunOpts{Detached: true, Name: "app", Ports: []string{"3000:3000"}, Command: "npm start"},
			[]string{"run", "-d", "--name", "app", "-p", "3000:3000",
				"-e", "E=1", "-v", "/m:/m", "-w", "/w",
				"--userns=keep-id", "--security-opt", "label=disable",
				"--cap-add", "SYS_PTRACE", "img", "/bin/sh", "-lc", "npm start"},
		},
		{
			"serve/apple", apple,
			RunOpts{Detached: true, Name: "app", Ports: []string{"3000:3000"}, Command: "npm start"},
			[]string{"run", "-d", "--name", "app", "-p", "3000:3000",
				"-e", "E=1", "-v", "/m:/m", "-w", "/w",
				"--cap-add", "SYS_PTRACE", "img", "/bin/sh", "-lc", "npm start"},
		},
		{
			"sandbox/podman", pod, RunOpts{Sandbox: true, Interactive: true},
			[]string{"run", "-it", "--rm", "-v", "/work:/work", "-w", "/work",
				"-e", "HOME=/tmp", "--read-only", "--tmpfs", "/tmp",
				"--cap-drop", "ALL", "--security-opt", "no-new-privileges",
				"--network", "none", "--userns=keep-id", "img", "/bin/sh", "-l"},
		},
		{
			"sandbox/apple", apple, RunOpts{Sandbox: true, Interactive: true},
			[]string{"run", "-it", "--rm", "-v", "/work:/work", "-w", "/work",
				"-e", "HOME=/tmp", "--read-only", "--tmpfs", "/tmp",
				"--cap-drop", "ALL", "--network", "none", "img", "/bin/sh", "-l"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := BuildRunArgs(tc.d, newCfg(), tc.o)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("BuildRunArgs mismatch\n got: %v\nwant: %v", got, tc.want)
			}
		})
	}
}

// TestModeArgs pins the run-mode flag sets: detached (serve) is named, interactive is a
// throwaway TTY, and a non-interactive command run keeps stdin open (-i) without a TTY so
// piped input / captured output stay clean.
func TestModeArgs(t *testing.T) {
	if got := modeArgs(RunOpts{Detached: true, Name: "app"}); !reflect.DeepEqual(got, []string{"-d", "--name", "app"}) {
		t.Errorf("detached modeArgs = %v, want [-d --name app]", got)
	}
	if got := modeArgs(RunOpts{Interactive: true}); !reflect.DeepEqual(got, []string{"-it", "--rm"}) {
		t.Errorf("interactive modeArgs = %v, want [-it --rm]", got)
	}
	// The piped (non-TTY) command branch returns -i --rm. It only fires when stdio are
	// not terminals; skip when the test itself runs attached to a TTY, where the code
	// would (correctly) return -it instead.
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		t.Skip("test stdio are TTYs; the non-TTY branch is unreachable here")
	}
	if got := modeArgs(RunOpts{Command: "echo hi"}); !reflect.DeepEqual(got, []string{"-i", "--rm"}) {
		t.Errorf("piped modeArgs = %v, want [-i --rm]", got)
	}
}

// TestExecArgs pins the two `exec` forms: an interactive login shell when no command is
// given, and a one-shot command run through the login shell otherwise.
func TestExecArgs(t *testing.T) {
	cfg := &config.Config{Shell: "/bin/sh"}
	if got := ExecArgs(cfg, "box", ""); !reflect.DeepEqual(got, []string{"exec", "-it", "box", "/bin/sh", "-l"}) {
		t.Errorf("interactive ExecArgs = %v, want [exec -it box /bin/sh -l]", got)
	}
	if got := ExecArgs(cfg, "box", "echo hi"); !reflect.DeepEqual(got, []string{"exec", "box", "/bin/sh", "-lc", "echo hi"}) {
		t.Errorf("command ExecArgs = %v, want [exec box /bin/sh -lc echo hi]", got)
	}
}

// A named runtime that is not known is a clear error listing the known runtimes.
func TestDetectUnknownRuntime(t *testing.T) {
	_, err := Detect("docker")
	if err == nil {
		t.Fatal("expected error for unknown runtime, got nil")
	}
	for _, name := range []string{"container", "podman"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should name known runtime %q: %v", name, err)
		}
	}
}
