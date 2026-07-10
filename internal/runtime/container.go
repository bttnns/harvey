package runtime

// appleContainer drives Apple's `container` CLI (macOS). It lists running containers
// with `ls` instead of `ps`, and supports a narrower sandbox flag set; everything
// else is the base default.
type appleContainer struct{ base }

func (appleContainer) ListArgs() []string { return []string{"ls"} }

// SandboxArgs for Apple `container`: it supports read-only rootfs, tmpfs, cap-drop, and
// `--network none`, but not no-new-privileges (still reported unsupported). `--network
// none` is accepted but undocumented upstream (docs describe only `--network <name>`);
// verified live 2026-07-09 on container 1.0.0 and 1.1.0, where the sealed container
// sees only loopback and both raw egress and DNS fail. The e2e suite carries an egress
// probe to catch an upstream regression; the per-container microVM adds its own kernel
// isolation.
func (appleContainer) SandboxArgs(network string) (args, unsupported []string) {
	args = []string{"--read-only", "--tmpfs", "/tmp", "--cap-drop", "ALL"}
	if network == "" || network == "none" {
		args = append(args, "--network", "none")
	} else {
		args = append(args, "--network", network)
	}
	unsupported = append(unsupported, "no-new-privileges")
	return args, unsupported
}
