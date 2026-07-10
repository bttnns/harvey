package cmd

import (
	"errors"
	"fmt"

	"github.com/bttnns/harvey/internal/runtime"
)

// Exit codes are part of harv's contract; the table lives in docs/spec.md. Commands
// that hand off with syscall.Exec (harv <cmd>, sandbox, exec, enter, ls, logs, cp)
// replace this process, so the wrapped command's own exit code propagates untouched and
// never reaches exitCode.
const (
	exitOK        = 0 // success
	exitError     = 1 // general harv failure
	exitUsage     = 2 // misuse: a bad flag/value or a mistyped management verb
	exitNoRuntime = 3 // no usable container runtime (none installed, or not on PATH)
)

// usageError marks a misuse (a bad flag or a management-verb typo) so Execute can map
// it to the conventional exit code 2 instead of the catch-all 1.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// usageErrorf builds a usageError from a format string.
func usageErrorf(format string, a ...any) error {
	return &usageError{fmt.Errorf(format, a...)}
}

// exitCode maps a top-level error to harv's exit-code contract. Anything not recognized
// as a runtime-missing or usage error is a general failure (1).
func exitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, runtime.ErrNoRuntime):
		return exitNoRuntime
	case isUsageError(err):
		return exitUsage
	default:
		return exitError
	}
}

func isUsageError(err error) bool {
	var ue *usageError
	return errors.As(err, &ue)
}
