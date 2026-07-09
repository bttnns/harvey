package runtime

// podman drives Podman. The mounted $HOME needs keep-id (so files written inside get
// the right ownership) and label=disable (SELinux on the bind mount); the rest is the
// base default.
type podman struct{ base }

func (podman) ExtraRunOpts() []string {
	return []string{"--userns=keep-id", "--security-opt", "label=disable"}
}
