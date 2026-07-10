# harvey spec

The canonical reference for the `.harvey.yaml` schema, the command set, and the
runtime drivers. The [README](../README.md) is the narrative intro and the
[usage guide](usage.md) is the task-oriented walkthrough; this is the exhaustive list
of options. The sandbox profile has its own deep dive in [sandbox.md](sandbox.md).

## How it works

```
        you type:  harv <something>
              │
              ▼
   ┌─────────────────────────────────────────────┐
   │  cmd/  (cobra command tree)                   │   what the user asked for
   │   root · exec · serve · lifecycle · image     │
   │   version · cp · doctor · scaffold            │
   └───────────────┬───────────────────────────────┘
                   │ resolve()
        ┌──────────┴───────────┐
        ▼                      ▼
┌────────────────┐   ┌──────────────────────────┐
│ internal/config│   │ internal/runtime         │   load config + pick runtime
│ load · merge   │   │  Driver interface        │
│ expand · valid │   │   base ─ container       │
│                │   │        ─ podman          │
└───────┬────────┘   └────────────┬─────────────┘
        └─────────────┬───────────┘
                      ▼
            BuildRunArgs(driver, cfg, opts)            assemble the command line
                      │
                      ▼
   Hand() = syscall.Exec ──► container | podman   (the real runtime)
                      │
                      ▼
            image + $HOME mounted at the same path + -w $PWD, throwaway (--rm)
```

Commands group into: **run code** (`harv`, `harv <cmd>`), **servers**
(`serve`/`exec`/`ls`/`logs`/`rm`/`cp`), **images** (`init`/`recreate`), and
**setup/diagnostics** (`scaffold`/`doctor`/`version`/`completion`).

## Configuration

### Where config comes from (lowest to highest precedence)

1. Built-in defaults (`shell: /bin/sh`, `runtime: auto`, `image:` = Red Hat UBI9)
2. `~/.config/harvey/config.yaml` (user-level)
3. The nearest `.harvey.yaml` found by walking up from `$PWD` (project-level)
4. `HARVEY_IMAGE` / `HARVEY_RUNTIME` environment variables
5. `--image` / `--runtime` / `--platform` / `--config` flags

Merging is per key: a key set in a higher layer wins; keys left unset inherit from
the layer below. `--config PATH` replaces the file search with a single explicit
file. Unknown keys are a hard error (typo protection).

### Keys

| Key | Type | Default | Description |
|---|---|---|---|
| `image` | string | `registry.access.redhat.com/ubi9/ubi:latest` | Image to run, and to build with `init`/`recreate`. A registry image is pulled on first run; only `build:`-based images need `harv init`. |
| `runtime` | string | `auto` | `auto` \| `container` \| `podman`. |
| `shell` | string | `/bin/sh` | Run as a login shell: `-l` interactively, `-lc "<cmd>"` for a command. |
| `platform` | string | none | `os/arch` for run + build, e.g. `linux/amd64` (emulation). |
| `workdir` | string | `$PWD` | Working directory inside the container. |
| `user` | string | none | `-u name\|uid[:gid]`. |
| `network` | string | none | `--network` mode. |
| `home` | bool | `true` | Dev mode only: bind-mount `$HOME` at the same path. `home: false` opts out. Sandbox mode never mounts `$HOME`. |
| `mounts` | list | none | `SRC:DST` bind mounts, added alongside the default `$HOME` mount (an explicit `$HOME:$HOME` here is not duplicated). |
| `env` | list | none | `KEY=VALUE` environment entries. |
| `envFile` | list | none | `--env-file` paths (repeatable). |
| `runArgs` | list | none | Raw flags appended verbatim to `run` (escape hatch). |
| `postBuild` | list | none | Commands run (throwaway, `$HOME`-mounted) after a build. |
| `build.containerfile` | string | none | Containerfile path; required for `init`/`recreate`. |
| `build.context` | string | none | Build context directory; required for `init`/`recreate`. |
| `build.args` | list | none | `--build-arg KEY=VALUE`. |
| `build.target` | string | none | `--target` build stage. |
| `sandbox.image` | string | falls back to `image` | Dedicated image for `harv sandbox` (typically leaner/locked-down). Unset uses `image`; `HARVEY_IMAGE`/`--image` override only `image`, so they reach a sandbox run only via this fallback, never over an explicit `sandbox.image`. |
| `sandbox.workspace` | string | `$PWD` | The only rw mount in sandbox mode (never `$HOME`). |
| `sandbox.network` | string | `none` | Sandbox network mode; `none` blocks egress. |
| `sandbox.env` | list | none | `KEY=VALUE` injected into the sandbox (e.g. an agent API key). |
| `sandbox.envFile` | list | none | `--env-file` paths for the sandbox. |
| `sandbox.memory` | string | none | `--memory` limit in sandbox mode. |
| `sandbox.cpus` | string | none | `--cpus` limit in sandbox mode. |

A leading `~` and any `$VAR` are expanded in path-like fields (`image`, `shell`,
`workdir`, `mounts`, `env`, `envFile`, `build` paths). See
[`example.harvey.yaml`](../example.harvey.yaml) for a documented sample.

## Commands

| Command | Flags | What it does |
|---|---|---|
| `harv` | | Interactive login shell in a throwaway container |
| `harv <cmd...>` | | Run a command in a throwaway container |
| `harv exec NAME [cmd...]` | | Shell/run a command **inside a running** container (Docker-style) |
| `harv serve <cmd...>` | `--name` (required), `-p PORT` (repeatable) | Detached, port-published container |
| `harv enter [NAME] [cmd...]` | | Enter (creating if absent) a PERSISTENT named container; a cache between rebuilds |
| `harv sandbox [cmd...]` | | Run a command/agent in the locked-down sandbox profile |
| `harv ls` (alias `ps`) | runtime-native flags pass through (e.g. `-a`) | List running containers |
| `harv logs NAME` | `-f` | Show or follow a container's logs |
| `harv rm NAME` | | Stop and remove a container |
| `harv cp SRC DST` | | Copy files between host and container (`NAME:path`) |
| `harv init` | `--build-arg`, `--target`, `--no-cache` | Build the image if it is not present |
| `harv recreate` | `--build-arg`, `--target`, `--no-cache` | Force a full image rebuild |
| `harv doctor` | | Check runtime, image, config, and platform |
| `harv scaffold` | `--force` | Write a starter `.harvey.yaml` |
| `harv version` | | harvey version + detected runtime (also `--version`) |
| `harv completion <shell>` | | Shell completion script (cobra built-in) |
| `harv help` | | Show usage |

Global flags (all commands): `--image IMAGE`, `--runtime RT`, `--platform OS/ARCH`,
`--config PATH`.

Notes:
- The throwaway one-off is the bare `harv <cmd...>`; `harv exec` targets a **running**
  container (like `docker exec`). Inner-command flags (`harv go test -v`) pass through.
- `-p 3000` is shorthand for `3000:3000`; `-p 8080:3000` passes through unchanged.
- A `build:`-based image is never built implicitly (`harv init` does that); a plain
  registry image (incl. the default UBI9) is pulled by the runtime on first run.
- `postBuild` commands run as throwaway containers after a successful build, so their
  writes land in the mounted `$HOME` (the image itself is not mutated).
- `harv enter` is the one persistent ("pet") mode: unlike every other run it is not
  `--rm`. With no NAME it derives one from the current directory (basename sanitized to
  container-name rules, prefixed `harvey-`, e.g. `~/Dev/foo` -> `harvey-foo`); a given
  NAME is used verbatim. A missing container is started detached with the dev profile
  and an idle keepalive process, a stopped one is restarted (its filesystem persists),
  and a running one is reused; then a login shell (or `[cmd...]`) is exec'd in. It shows
  in `harv ls` and is removed with `harv rm NAME` like any named container.

## Exit codes

`harv`'s exit status is part of its contract, so scripts and agents can branch on it.

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | General `harv` failure (config parse error, image build failed, a runtime command returned an error, etc.). |
| `2` | Usage error: an unknown/invalid flag or value, or a mistyped management verb (`harv stop` -> "did you mean `harv rm`"). |
| `3` | No usable container runtime: none installed, or the one named by `runtime:` / `--runtime` is not on `PATH`. (A runtime that is on `PATH` but fails to connect surfaces the runtime's own error and exit code, not this.) |

**Passthrough.** Every command that hands off to the runtime with `syscall.Exec`
(`harv <cmd>`, `harv sandbox`, `harv exec`, `harv enter`, `harv ls`, `harv logs`,
`harv cp`) *replaces* the `harv` process with the runtime, so the wrapped command's own
exit code propagates untouched. `harv go test ./...` exits with `go test`'s status;
`harv sandbox claude` exits with the agent's status. The codes above apply only to
failures `harv` itself detects before or instead of that handoff.

Idempotent cleanup: `harv rm NAME` on a container that does not exist prints a one-line
note and exits `0` (removing something already gone is success, not an error). `harv
init` when the image is already present is a fast no-op that also exits `0`; `harv
recreate` always force-rebuilds.

## Output styling and color

Help, usage, the `--version` line, and error messages are rendered with
[`fang`](https://charm.land/fang) (styled boxes on a terminal, a build-info `--version`,
a hidden `man` command, and shell `completion`). Coloring honors the informal terminal
color standards via `colorprofile`:

- `NO_COLOR` (any non-empty value) disables color. Per the standard it still allows text
  decoration (bold/faint), so a `NO_COLOR` run is monochrome, not plain-ASCII.
- `CLICOLOR=0` disables color; `CLICOLOR_FORCE=1` forces it even when output is not a
  terminal. `NO_COLOR` takes precedence over both.
- `TERM=dumb` is treated as non-color unless `CLICOLOR_FORCE=1`.
- Output to a pipe or file (not a TTY) is never colored unless forced. `harv --help |
  cat` emits no escape sequences.

(`FORCE_COLOR`, a Node.js convention, is not consulted; use `CLICOLOR_FORCE=1`.)

## Runtime drivers

`harv` runs natively on Apple `container` (macOS) and Podman (Linux).

| Runtime | Binary | List verb | Dev `run` extras | Sandbox `run` extras |
|---|---|---|---|---|
| Apple `container` | `container` | `ls` | none | none |
| Podman | `podman` | `ps` | `--userns=keep-id --security-opt label=disable` | `--userns=keep-id` |

The driver is the command-translation layer: `harv` exposes one canonical verb and
each driver emits the runtime's native form. `run`, `exec`, `cp`, `rm`, `logs`,
`build`, and `image inspect` are identical across both; the only verb that needs
translating is list (`ls` vs `ps`), and `harv` accepts either spelling. `--platform`
works on both.

The dev extras go on every trusted dev-mode run; the sandbox extras go on every
`harv sandbox` run instead, and never inherit the dev extras (Podman's dev
`label=disable` turns SELinux off, which must not leak into a locked-down run) or the
`runArgs` escape hatch. See [sandbox.md](sandbox.md).

Auto-detection tries `container`, then `podman`, and uses the first found on `PATH`.
Override with the `runtime:` key or `--runtime`. Each runtime is a `Driver` in
[`internal/runtime`](../internal/runtime): a type that embeds `base` and overrides only
what differs, plus one entry in the `drivers` detection slice. Adding a runtime is one
new file plus one line.

**Docker is not supported right now.** It was dropped to keep the surface focused on the
two runtimes actually in use. The pluggable `Driver` interface is the seam it left
behind: a Docker driver can be contributed or revived on demand as one new file plus one
line in the `drivers` slice, with nothing else in the codebase changing.

## Sandboxing AI agents

The `sandbox.*` keys above configure `harv sandbox`, a locked-down profile for
untrusted code and autonomous AI agents that inverts the permissive dev default: same
image, but none of your host. Its threat model, per-runtime support matrix, and
per-agent recipes have a dedicated deep dive in [sandbox.md](sandbox.md).
