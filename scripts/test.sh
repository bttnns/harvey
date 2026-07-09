#!/usr/bin/env bash
# Automated end-to-end test for harvey.
#
# Builds harv (if a Go toolchain is present, else uses a prebuilt ./harv), then
# exercises the core commands against one or more runtimes. Everything is created
# under a temp workspace and named with a fixed prefix, and a trap on EXIT removes
# the test container, the test image, and the temp workspace EVEN WHEN A STEP FAILS.
#
# Usage:
#   scripts/test.sh                 # test every runtime found on PATH
#   scripts/test.sh docker podman   # test only the named runtimes
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HARV="$REPO/harv"
IMAGE="harvey-selftest"
CNAME="harvey-selftest-srv"
PORT="${PORT:-8095}"
WORK=""

RUNTIMES=("$@")
if [ "${#RUNTIMES[@]}" -eq 0 ]; then
  for r in container podman docker; do
    command -v "$r" >/dev/null 2>&1 && RUNTIMES+=("$r")
  done
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
  "$HARV" --runtime "$RT" recreate --build-arg FOO=hi >/dev/null
  [ "$(cat postbuild.out)" = "ok" ];                                  ok "init + postBuild hook"
  [ "$("$HARV" --runtime "$RT" cat /foo.txt)" = "FOO=hi" ];           ok "build-arg"
  [ "$("$HARV" --runtime "$RT" uname -s)" = "Linux" ];                ok "throwaway run"
  "$HARV" --runtime "$RT" ls >/dev/null
  "$HARV" --runtime "$RT" ps >/dev/null;                              ok "ls + ps alias"
  "$HARV" --runtime "$RT" serve --name "$CNAME" -p "$PORT" python3 -m http.server "$PORT" >/dev/null
  sleep 3
  [ "$("$HARV" --runtime "$RT" exec "$CNAME" echo inside)" = "inside" ]; ok "exec into running container"
  [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$PORT/")" = "200" ]; ok "serve published port"
  "$HARV" --runtime "$RT" rm "$CNAME" >/dev/null;                     ok "rm"
  "$HARV" --runtime "$RT" doctor >/dev/null;                          ok "doctor"
  # Sandbox: the root filesystem must be read-only (a portable check across all
  # three runtimes; network isolation is asserted for docker/podman only).
  if "$HARV" --runtime "$RT" sandbox 'touch /sandbox-probe' >/dev/null 2>&1; then
    echo "sandbox rootfs is writable on $RT"; exit 1
  fi
  ok "sandbox (read-only rootfs)"
  if [ "$RT" != "container" ]; then
    if "$HARV" --runtime "$RT" sandbox 'cat /proc/net/route | grep -qv Iface' >/dev/null 2>&1; then
      echo "sandbox has routes on $RT (expected none)"; exit 1
    fi
    ok "sandbox (network isolated)"
  fi
done

# Strict decode must reject unknown keys.
printf 'image: x\nbogus_key: 1\n' > "$WORK/bad.yaml"
if "$HARV" --config "$WORK/bad.yaml" ls >/dev/null 2>&1; then
  echo "strict decode did NOT reject an unknown key"; exit 1
fi
ok "strict decode rejects unknown keys"
