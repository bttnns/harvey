#!/usr/bin/env bash
# Local end-to-end gate for the Mac (the AUTHORITATIVE pre-merge check).
#
# Runs scripts/test.sh against every supported runtime present on this machine:
# Apple `container` always, and Podman as well when it is installed. Each runtime
# is run to completion even if an earlier one fails, then a per-runtime PASS/FAIL
# summary is printed. Exits nonzero if any runtime failed.
#
# Usage:
#   scripts/e2e-all.sh
set -uo pipefail  # deliberately NOT -e: run every runtime, collect all results.

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Apple `container` is the primary Mac runtime; Podman is included only when it is
# actually usable (`podman info`), not merely on PATH: on macOS the Homebrew podman
# client without a started machine would otherwise produce a false FAIL.
RUNTIMES=(container)
if command -v podman >/dev/null 2>&1; then
  if podman info >/dev/null 2>&1; then
    RUNTIMES+=(podman)
  else
    echo "podman: SKIP (installed but not usable; is the podman machine running?)"
  fi
fi

RESULTS=()
FAIL=0
for RT in "${RUNTIMES[@]}"; do
  echo "==================== e2e: $RT ===================="
  if "$DIR/test.sh" "$RT"; then
    RESULTS+=("$RT: PASS")
  else
    RESULTS+=("$RT: FAIL")
    FAIL=1
  fi
  echo
done

echo "==================== e2e summary ===================="
for r in "${RESULTS[@]}"; do
  echo "  $r"
done

exit "$FAIL"
