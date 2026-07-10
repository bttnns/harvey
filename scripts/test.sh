#!/usr/bin/env bash
# Automated end-to-end test for harvey.
#
# Builds harv (if a Go toolchain is present, else uses a prebuilt ./harv), then
# exercises the core commands against one or more runtimes. Everything is created
# under a temp workspace and named with a fixed prefix, and a trap on EXIT removes
# the test container, the test image, and the temp workspace EVEN WHEN A STEP FAILS.
#
# Usage:
#   scripts/test.sh                 # auto-detect: container, then podman
#   scripts/test.sh podman          # test only the named runtime(s)
#   HARVEY_RUNTIME=podman scripts/test.sh   # pick the runtime via env
#
# The runtime may be given as positional args ($1 ...), via HARVEY_RUNTIME, or left
# to auto-detection (container preferred, then podman). Docker is no longer supported.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HARV="$REPO/harv"
IMAGE="harvey-selftest"
CNAME="harvey-selftest-srv"
PORT="${PORT:-8095}"
WORK=""

RUNTIMES=("$@")
if [ "${#RUNTIMES[@]}" -eq 0 ]; then
  if [ -n "${HARVEY_RUNTIME:-}" ]; then
    RUNTIMES=("$HARVEY_RUNTIME")
  else
    for r in container podman; do
      command -v "$r" >/dev/null 2>&1 && RUNTIMES+=("$r")
    done
  fi
fi
[ "${#RUNTIMES[@]}" -gt 0 ] || { echo "no container runtime found"; exit 1; }

cleanup() {
  local code=$?
  for r in "${RUNTIMES[@]}"; do
    "$HARV" --runtime "$r" rm "$CNAME"  >/dev/null 2>&1 || true
    "$r" image rm "$IMAGE"              >/dev/null 2>&1 || true
  done
  [ -n "$WORK" ] && rm -rf "$WORK"
  if [ "$code" -eq 0 ]; then echo "PASS"; else echo "FAIL (exit $code)"; fi
  exit "$code"
}
trap cleanup EXIT

ok() { printf '  ok  %s\n' "$1"; }

# Build the host binary, or fall back to a prebuilt one.
if command -v go >/dev/null 2>&1; then
  ( cd "$REPO" && go build -o harv . ); ok "build"
elif [ -x "$HARV" ]; then
  ok "using prebuilt harv"
else
  echo "no Go toolchain and no prebuilt ./harv"; exit 1
fi

# Temp workspace under $HOME so the $HOME bind mount covers it.
WORK="$(mktemp -d "$HOME/.harvey-selftest.XXXXXX")"
cat > "$WORK/Containerfile" <<'EOF'
FROM python:3-slim
ARG FOO=default
RUN echo "FOO=$FOO" > /foo.txt
EOF
cat > "$WORK/.harvey.yaml" <<'EOF'
image: harvey-selftest
shell: /bin/sh
mounts:
  - $HOME:$HOME
env:
  - HOME=$HOME
build:
  containerfile: ./Containerfile
  context: .
postBuild:
  - echo ok > postbuild.out
EOF
cd "$WORK"

for RT in "${RUNTIMES[@]}"; do
  echo "== runtime: $RT =="
  "$HARV" --runtime "$RT" rm "$CNAME" >/dev/null 2>&1 || true
  # NOTE: send container-STARTING commands' stdout to a log file, never /dev/null.
  # Apple `container` 1.0.0 cannot wire a container's stdio to /dev/null and fails the
  # start with ENODEV ("Operation not supported by device"); a regular file is fine.
  # Fixed upstream in 1.1.0 (verified 2026-07-09); kept for anyone still on 1.0.0.
  # (Commands that only query the daemon, ls/ps/rm/doctor, are safe with /dev/null.)
  LOG="$WORK/harv.log"
  "$HARV" --runtime "$RT" recreate --build-arg FOO=hi >"$LOG"
  [ "$(cat postbuild.out)" = "ok" ];                                  ok "init + postBuild hook"
  [ "$("$HARV" --runtime "$RT" cat /foo.txt)" = "FOO=hi" ];           ok "build-arg"
  [ "$("$HARV" --runtime "$RT" uname -s)" = "Linux" ];                ok "throwaway run"
  "$HARV" --runtime "$RT" ls >/dev/null
  "$HARV" --runtime "$RT" ps >/dev/null;                              ok "ls + ps alias"
  "$HARV" --runtime "$RT" serve --name "$CNAME" -p "$PORT" python3 -m http.server "$PORT" >"$LOG"
  sleep 3
  [ "$("$HARV" --runtime "$RT" exec "$CNAME" echo inside)" = "inside" ]; ok "exec into running container"
  [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$PORT/")" = "200" ]; ok "serve published port"
  "$HARV" --runtime "$RT" rm "$CNAME" >/dev/null;                     ok "rm"
  "$HARV" --runtime "$RT" doctor >/dev/null;                          ok "doctor"
  # Sandbox: the root filesystem must be read-only (a portable check across every
  # supported runtime).
  if "$HARV" --runtime "$RT" sandbox 'touch /sandbox-probe' >/dev/null 2>&1; then
    echo "sandbox rootfs is writable on $RT"; exit 1
  fi
  ok "sandbox (read-only rootfs)"
  # Low-level route check on the OCI runtimes: a sealed sandbox sees no routes.
  # Apple `container` runs a per-VM kernel where /proc/net/route may differ, so it
  # relies on the egress probe below instead.
  if [ "$RT" != "container" ]; then
    if "$HARV" --runtime "$RT" sandbox 'cat /proc/net/route | grep -qv Iface' >/dev/null 2>&1; then
      echo "sandbox has routes on $RT (expected none)"; exit 1
    fi
    ok "sandbox (network isolated)"
  fi

  # Egress probe (portable, all runtimes): a sealed sandbox must reach NOTHING, by
  # DNS name or by raw IP. Reuses the selftest image's python3 (no extra tooling);
  # urllib.urlopen raises on any failure, so a nonzero exit prints SEALED. This
  # catches an upstream regression in Apple `container`'s undocumented `--network
  # none`, which is the only thing sealing the microVM's egress.
  for target in http://example.com http://1.1.1.1; do
    verdict="$("$HARV" --runtime "$RT" sandbox \
      "python3 -c \"import urllib.request; urllib.request.urlopen('$target', timeout=3)\" 2>/dev/null && echo OPEN || echo SEALED")"
    if [ "$verdict" != "SEALED" ]; then
      echo "sandbox egress to $target was '$verdict' on $RT (expected SEALED)"; exit 1
    fi
  done
  ok "sandbox (egress sealed: DNS + raw IP)"

  # runArgs escape-hatch probe: a project's raw `runArgs` must apply to a trusted dev
  # run but be IGNORED by a sandbox run, so config can never smuggle a host mount past
  # the fence. A canary dir is bind-mounted to /canary via runArgs: visible in dev,
  # absent in sandbox.
  mkdir -p "$WORK/canary"
  echo canary > "$WORK/canary/marker"
  cat > "$WORK/runargs.yaml" <<EOF
image: $IMAGE
shell: /bin/sh
workdir: /
runArgs:
  - -v
  - $WORK/canary:/canary
EOF
  [ "$("$HARV" --config "$WORK/runargs.yaml" --runtime "$RT" cat /canary/marker)" = "canary" ]
  ok "runArgs applied to dev run"
  leak="$("$HARV" --config "$WORK/runargs.yaml" --runtime "$RT" \
    sandbox 'ls /canary >/dev/null 2>&1 && echo LEAK || echo SEALED')"
  if [ "$leak" != "SEALED" ]; then
    echo "sandbox honored the runArgs escape hatch on $RT (/canary present)"; exit 1
  fi
  ok "runArgs ignored in sandbox run"
done

# Strict decode must reject unknown keys.
printf 'image: x\nbogus_key: 1\n' > "$WORK/bad.yaml"
if "$HARV" --config "$WORK/bad.yaml" ls >/dev/null 2>&1; then
  echo "strict decode did NOT reject an unknown key"; exit 1
fi
ok "strict decode rejects unknown keys"
