package runtime

// podman drives Podman. In dev mode the mounted $HOME needs keep-id (so files written
// inside get the right ownership) and label=disable (SELinux off on the bind mount);
// the rest is the base default.
type podman struct{ base }

func (podman) ExtraRunOpts() []string {
	return []string{"--userns=keep-id", "--security-opt", "label=disable"}
}

// SandboxRunOpts is the sandbox counterpart, and deliberately NOT ExtraRunOpts. keep-id
// still maps the container user to the host user so it can write the mounted workspace
// with matching ownership, but label=disable is dropped: turning SELinux off would
// weaken the fence, and the sandbox mounts only the workspace (not $HOME) so the label
// friction it works around does not arise.
func (podman) SandboxRunOpts() []string {
	return []string{"--userns=keep-id"}
}
