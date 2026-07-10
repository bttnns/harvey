package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes content to path, creating parent dirs, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// chdir switches to dir for the duration of the test, restoring the old cwd after.
// (Go 1.23 has no t.Chdir.)
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// TestHomeOverlay pins the overlay semantics of the *bool `home` key: an unset key in
// a higher layer inherits the layer below (it does not clobber it with false), while a
// key that is present wins. Load reads the user file (~/.config/harvey/config.yaml via
// $HOME) first, then overlays the nearest project .harvey.yaml, so this drives the real
// Load path by pointing $HOME at a temp dir and running from a temp project dir.
func TestHomeOverlay(t *testing.T) {
	cases := []struct {
		name    string
		user    string // ~/.config/harvey/config.yaml contents
		project string // ./.harvey.yaml contents
		want    bool
	}{
		{"both unset defaults true", "", "", true},
		{"user false, project unset stays false", "home: false\n", "", false},
		{"project true overrides user false", "home: false\n", "home: true\n", true},
		{"user true, project false overrides", "home: true\n", "home: false\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := t.TempDir()
			projDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			writeFile(t, filepath.Join(homeDir, ".config", "harvey", "config.yaml"), tc.user)
			writeFile(t, filepath.Join(projDir, ".harvey.yaml"), tc.project)
			chdir(t, projDir)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.HomeMount(); got != tc.want {
				t.Errorf("HomeMount() = %v, want %v (user=%q project=%q)", got, tc.want, tc.user, tc.project)
			}
		})
	}
}

// TestSandboxImageFallback pins the fallback chain: an explicit sandbox.image wins, an
// unset one falls back to the top-level image, and the project file overlays the user
// file for both keys just like every other field.
func TestSandboxImageFallback(t *testing.T) {
	cases := []struct {
		name    string
		user    string
		project string
		want    string
	}{
		{"unset falls back to image", "image: main\n", "", "main"},
		{"set wins over image", "image: main\nsandbox:\n  image: lean\n", "", "lean"},
		{"project overrides user image (fallback tracks it)", "image: main\n", "image: proj\n", "proj"},
		{"project overrides user sandbox.image", "sandbox:\n  image: user\n", "sandbox:\n  image: proj\n", "proj"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			homeDir := t.TempDir()
			projDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			writeFile(t, filepath.Join(homeDir, ".config", "harvey", "config.yaml"), tc.user)
			writeFile(t, filepath.Join(projDir, ".harvey.yaml"), tc.project)
			chdir(t, projDir)

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.SandboxImage(); got != tc.want {
				t.Errorf("SandboxImage() = %q, want %q (user=%q project=%q)", got, tc.want, tc.user, tc.project)
			}
		})
	}
}

// TestSandboxImageExpands confirms sandbox.image gets the same ~ / $VAR expansion the
// top-level image does.
func TestSandboxImageExpands(t *testing.T) {
	t.Setenv("SBX_IMG", "lean-image")
	dir := t.TempDir()
	p := filepath.Join(dir, ".harvey.yaml")
	writeFile(t, p, "sandbox:\n  image: $SBX_IMG\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Expand()
	if got := cfg.SandboxImage(); got != "lean-image" {
		t.Errorf("SandboxImage() = %q, want %q", got, "lean-image")
	}
}

// TestLoadRejectsUnknownKey confirms strict decoding still rejects a typo'd key after
// adding `home`; a bogus key must be a hard error, not silently ignored.
func TestLoadRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".harvey.yaml")
	writeFile(t, p, "bogus: 1\n")
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

// TestLoadAcceptsHomeKey confirms `home:` is a recognized key (not caught by strict
// decode) and that an explicit value is read through.
func TestLoadAcceptsHomeKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".harvey.yaml")
	writeFile(t, p, "home: false\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HomeMount() {
		t.Errorf("expected HomeMount() false with home: false, got true")
	}
}

// TestLayeringProjectWins pins the core overlay contract across the two real config
// layers: the project .harvey.yaml overlays the user file per key. A key the project
// sets wins over the user's value; a key the project leaves unset keeps the user's
// value (rather than reverting to a built-in default).
func TestLayeringProjectWins(t *testing.T) {
	homeDir := t.TempDir()
	projDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	// User sets image + shell; project overrides only image and adds network.
	writeFile(t, filepath.Join(homeDir, ".config", "harvey", "config.yaml"),
		"image: userimg\nshell: /bin/bash\n")
	writeFile(t, filepath.Join(projDir, ".harvey.yaml"),
		"image: projimg\nnetwork: mynet\n")
	chdir(t, projDir)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Image != "projimg" {
		t.Errorf("Image = %q, want project value %q", cfg.Image, "projimg")
	}
	if cfg.Shell != "/bin/bash" {
		t.Errorf("Shell = %q, want retained user value %q", cfg.Shell, "/bin/bash")
	}
	if cfg.Network != "mynet" {
		t.Errorf("Network = %q, want project-only value %q", cfg.Network, "mynet")
	}
}

// TestEnvOverrides confirms HARVEY_IMAGE and HARVEY_RUNTIME override whatever the files
// resolved to (they are applied last, after both layers are merged).
func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".harvey.yaml")
	writeFile(t, p, "image: fileimg\nruntime: podman\n")
	t.Setenv("HARVEY_IMAGE", "envimg")
	t.Setenv("HARVEY_RUNTIME", "container")

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Image != "envimg" {
		t.Errorf("HARVEY_IMAGE not applied: Image = %q, want %q", cfg.Image, "envimg")
	}
	if cfg.Runtime != "container" {
		t.Errorf("HARVEY_RUNTIME not applied: Runtime = %q, want %q", cfg.Runtime, "container")
	}
}

// TestFindProjectConfigWalksUp confirms Load discovers the nearest .harvey.yaml by
// walking up from the working directory, so `harv` works from any subdirectory. The
// config lives at the project root while cwd is a nested subdir; the user layer is
// absent (empty $HOME) so any value read must have come from the discovered project file.
func TestFindProjectConfigWalksUp(t *testing.T) {
	homeDir := t.TempDir()
	projDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeFile(t, filepath.Join(projDir, ".harvey.yaml"), "image: rootimg\n")
	sub := filepath.Join(projDir, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, sub)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Image != "rootimg" {
		t.Errorf("walk-up did not find root .harvey.yaml: Image = %q, want %q", cfg.Image, "rootimg")
	}
}

// TestExpandImageMountsEnv confirms Expand resolves a leading ~ and $VARs across the
// image, mounts, and env fields (not just sandbox.image, which is covered separately).
func TestExpandImageMountsEnv(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("IMG", "myimg")
	t.Setenv("DATADIR", "/data")
	t.Setenv("TOKEN", "secret")

	dir := t.TempDir()
	p := filepath.Join(dir, ".harvey.yaml")
	writeFile(t, p, "image: $IMG\nmounts:\n  - ~/proj:/proj\n  - $DATADIR:/data\nenv:\n  - KEY=$TOKEN\n")

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Expand()

	if cfg.Image != "myimg" {
		t.Errorf("image $VAR not expanded: %q", cfg.Image)
	}
	wantMounts := []string{filepath.Join(homeDir, "proj") + ":/proj", "/data:/data"}
	if len(cfg.Mounts) != 2 || cfg.Mounts[0] != wantMounts[0] || cfg.Mounts[1] != wantMounts[1] {
		t.Errorf("mounts = %v, want %v (~ and $VAR expanded)", cfg.Mounts, wantMounts)
	}
	if len(cfg.Env) != 1 || cfg.Env[0] != "KEY=secret" {
		t.Errorf("env = %v, want [KEY=secret]", cfg.Env)
	}
}
