package runtime

// appleContainer drives Apple's `container` CLI (macOS). It lists running containers
// with `ls` instead of `ps`, and supports a narrower sandbox flag set; everything
// else is the base default.
type appleContainer struct{ base }

func (appleContainer) ListArgs() []string { return []string{"ls"} }

// SandboxArgs for Apple `container`: it supports read-only rootfs, tmpfs, and
// cap-drop, but not no-new-privileges, and has no documented `--network none`. Those
// gaps are reported as unsupported; the per-container microVM still isolates the
// kernel from the host.
func (appleContainer) SandboxArgs(network string) (args, unsupported []string) {
	args = []string{"--read-only", "--tmpfs", "/tmp", "--cap-drop", "ALL"}
	if network == "" || network == "none" {
		unsupported = append(unsupported, "network isolation (--network none)")
	} else {
		args = append(args, "--network", network)
	}
	unsupported = append(unsupported, "no-new-privileges")
	return args, unsupported
}
