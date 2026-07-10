package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"foo":           "foo",
		"My-Project":    "My-Project",  // case and single dashes preserved
		"foo_bar.baz":   "foo_bar.baz", // underscores and dots are legal
		"a b c":         "a-b-c",       // spaces become dashes
		"weird!!name":   "weird-name",  // runs of illegal chars collapse to one dash
		"--edge--":      "edge",        // leading/trailing separators trimmed
		"..dotty..":     "dotty",       // dots trimmed at the edges too
		"path/to/thing": "path-to-thing",
		"":              "",    // empty stays empty (caller supplies a fallback)
		"///":           "",    // all-illegal collapses then trims to empty
		"café":          "caf", // non-ASCII letters are not allowed, trimmed
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeriveName(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "My Cool.Project!")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(proj); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	got := deriveName()
	want := "harvey-My-Cool.Project"
	if got != want {
		t.Errorf("deriveName() in %q = %q, want %q", proj, got, want)
	}
}
