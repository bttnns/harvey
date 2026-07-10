// harvey (harv) is a small wrapper over a container runtime (Apple `container` on
// macOS, Podman on Linux) for throwaway, $HOME-mounted dev environments. The toolchain
// lives in a container image; harv runs commands inside it against your real files,
// then throws the container away (--rm) while caches under ~/.cache persist.
//
// Everything is driven by a .harvey.yaml file, so the same tool works for any
// image/mount/env setup, not one hardcoded machine.
package main

import "github.com/bttnns/harvey/cmd"

func main() {
	cmd.Execute()
}
