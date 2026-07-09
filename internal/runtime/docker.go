package runtime

// docker drives Docker. The base defaults (list with `ps`, no extra run opts) are
// exactly right, so it overrides nothing; it exists as its own type for symmetry and
// so the supported runtimes are each discoverable in one place.
type docker struct{ base }
