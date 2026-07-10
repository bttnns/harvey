package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bttnns/harvey/internal/runtime"
)

// exitCode maps each recognized error class to its contract code, and everything else
// to the catch-all 1. Wrapped errors must still be classified (errors.Is/As), so the
// guidance-carrying wrappers around ErrNoRuntime keep their code.
func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, exitOK},
		{"general", errors.New("boom"), exitError},
		{"usage", usageErrorf("bad flag"), exitUsage},
		{"usage-wrapped", fmt.Errorf("context: %w", usageErrorf("bad")), exitUsage},
		{"no-runtime", runtime.ErrNoRuntime, exitNoRuntime},
		{"no-runtime-wrapped", fmt.Errorf("none on PATH: %w", runtime.ErrNoRuntime), exitNoRuntime},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCode(tc.err); got != tc.want {
				t.Errorf("exitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// Every management-verb alias must point at a command that actually exists in the tree,
// so the suggestion never names a bogus command. Also pins that ordinary toolchain verbs
// (the whole point of harv's passthrough) are NOT intercepted.
func TestVerbAliases(t *testing.T) {
	commands := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		commands[c.Name()] = true
	}
	for alias, target := range verbAliases {
		if !commands[target] {
			t.Errorf("verbAliases[%q] = %q, which is not a registered command", alias, target)
		}
	}
	for _, keep := range []string{"go", "make", "npm", "python", "cat", "bash", "ls"} {
		if _, hijacked := verbAliases[keep]; hijacked {
			t.Errorf("verbAliases must not intercept the toolchain verb %q", keep)
		}
	}
}
