package runtime

import (
	"strings"
	"testing"
)

// Apple `container` now seals the network with `--network none` (accepted but
// undocumented upstream, verified live 2026-07-09 on 1.0.0). SandboxArgs must emit it for
// the default/none mode, pass a custom network through, and report only no-new-privileges
// as unsupported, never network isolation.
func TestAppleContainerSandboxArgs(t *testing.T) {
	d := appleContainer{base{name: "container", binary: "container"}}

	for _, network := range []string{"", "none"} {
		args, unsupported := d.SandboxArgs(network)
		if !contains(args, "--network") || !contains(args, "none") {
			t.Errorf("network %q: expected sealed --network none, got args %v", network, args)
		}
		u := joined(unsupported)
		if strings.Contains(u, "network") {
			t.Errorf("network %q: network must no longer be reported unsupported: %v", network, unsupported)
		}
		if !strings.Contains(u, "no-new-privileges") {
			t.Errorf("network %q: no-new-privileges should still be unsupported: %v", network, unsupported)
		}
	}

	// A custom network name passes through unchanged.
	args, _ := d.SandboxArgs("mynet")
	s := joined(args)
	if !strings.Contains(s, "--network mynet") {
		t.Errorf("custom network should pass through: %v", args)
	}
}
