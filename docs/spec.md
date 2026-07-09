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
│                │   │        ─ podman ─ docker │
└───────┬────────┘   └────────────┬─────────────┘
        └─────────────┬───────────┘
                      ▼
            BuildRunArgs(driver, cfg, opts)            assemble the command line
                      │
                      ▼
   Hand() = syscall.Exec ──► container | podman | docker   (the real runtime)
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
| `runtime` | string | `auto` | `auto` \| `container` \| `podman` \| `docker`. |
| `shell` | string | `/bin/sh` | Run as a login shell: `-l` interactively, `-lc "<cmd>"` for a command. |
| `platform` | string | none | `os/arch` for run + build, e.g. `linux/amd64` (emulation). |
| `workdir` | string | `$PWD` | Working directory inside the container. |
| `user` | string | none | `-u name\|uid[:gid]`. |
| `network` | string | none | `--network` mode. |
| `mounts` | list | none | `SRC:DST` bind mounts. |
| `env` | list | none | `KEY=VALUE` environment entries. |
| `envFile` | list | none | `--env-file` paths (repeatable). |
| `runArgs` | list | none | Raw flags appended verbatim to `run` (escape hatch). |
| `postBuild` | list | none | Commands run (throwaway, `$HOME`-mounted) after a build. |
| `build.containerfile` | string | none | Containerfile path; required for `init`/`recreate`. |
| `build.context` | string | none | Build context directory; required for `init`/`recreate`. |
| `build.args` | list | none | `--build-arg KEY=VALUE`. |
| `build.target` | string | none | `--target` build stage. |
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
| `harv sandbox [cmd...]` | | Run a command/agent in the locked-down sandbox profile |
| `harv ls` (alias `ps`) | runtime-native flags pass through (e.g. `-a`) | List running containers |
| `harv logs NAME` | `-f` | Show or follow a container's logs |
| `harv rm NAME` | | Stop and remove a container |
| `harv cp SRC DST` | | Copy files between host and container (`NAME:path`) |
| `harv init` | `--build-arg`, `--target`, `--no-cache` | Build the image if it is not present |
| `harv recreate` | `--build-arg`, `--target`, `--no-cache` | Force a full image rebuild |
| `harv doctor` | | Check runtime, image, config, and platform |
| `harv scaffold` | `--force` | Write a starter `.harvey.yaml` |
| `harv version` | | devctl/harvey version + detected runtime (also `--version`) |
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

## Runtime drivers

| Runtime | Binary | List verb | Extra `run` options |
|---|---|---|---|
| Apple `container` | `container` | `ls` | none |
| Podman | `podman` | `ps` | `--userns=keep-id --security-opt label=disable` |
| Docker | `docker` | `ps` | none |

The driver is the command-translation layer: `harv` exposes one canonical verb and
each driver emits the runtime's native form. `run`, `exec`, `cp`, `rm`, `logs`,
`build`, and `image inspect` are identical across all three; the only verb that needs
translating is list (`ls` vs `ps`), and `harv` accepts either spelling. `--platform`
works on all three.

Auto-detection tries `container`, then `podman`, then `docker`, and uses the first
found on `PATH`. Override with the `runtime:` key or `--runtime`. Each runtime is a
`Driver` in [`internal/runtime`](../internal/runtime): a type that embeds `base` and
overrides only what differs, plus one entry in the `drivers` detection slice. Adding
a runtime is one new file plus one line.

## Sandboxing AI agents

The `sandbox.*` keys above configure `harv sandbox`, a locked-down profile for
untrusted code and autonomous AI agents that inverts the permissive dev default: same
image, but none of your host. Its threat model, per-runtime support matrix, and
per-agent recipes have a dedicated deep dive in [sandbox.md](sandbox.md).
