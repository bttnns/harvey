# Usage guide

Task-oriented walkthrough of configuring and running `harv`. For the exhaustive schema
and command/flag reference, see [spec.md](spec.md); for the locked-down agent profile,
see [sandbox.md](sandbox.md).

## Configure

Copy [`example.harvey.yaml`](../example.harvey.yaml) to `.harvey.yaml` in a project (or
to `~/.config/harvey/config.yaml` for a global default) and edit:

```yaml
image: dotfiles-dev
shell: /usr/bin/zsh
mounts:
  - $HOME:$HOME
env:
  - HOME=$HOME
build:
  containerfile: $HOME/.config/dotfiles/Containerfile
  context: $HOME/.config/dotfiles
```

`image:` is the only required key (it defaults to Red Hat UBI9 if unset). The project
file overlays the user-level `~/.config/harvey/config.yaml`, and `~` and `$VAR` are
expanded in every path. See [spec.md](spec.md) for all keys, defaults, and the full
precedence order. `harv scaffold` writes a starter `.harvey.yaml` for you.

## Run code

```sh
harv                       # interactive login shell in a throwaway container
harv go test ./...         # run one command in a throwaway container
harv 'npm ci && npm test'  # quote to chain shell commands
harv --platform linux/amd64 go build ./...   # run under amd64 emulation
```

Every run is `--rm`: the container is discarded when the command exits, but your
bind-mounted `$HOME` (projects, dotfiles, `~/.cache`) persists.

## Build the image

```sh
harv init                  # build the image (no-op if it already exists)
harv recreate              # force a full rebuild
```

A `build:`-based image is never built implicitly; `harv` tells you to run `harv init`. A
plain registry image (including the default UBI9) is pulled by the runtime on first run.

## Browser-facing servers

A plain `harv` run publishes no ports. For a long-lived, port-published server, use
`harv serve`:

```sh
harv serve --name app -p 3000 npm run dev -- -H 0.0.0.0 -p 3000
harv ls                    # list running containers (alias: harv ps)
harv exec app bash         # open a shell INSIDE the running container
harv cp app:/out ./out     # copy files in/out of a container
harv logs app -f           # follow its logs
harv rm app                # stop and remove it
```

Two gotchas for `harv serve`:

- **Bind the server to `0.0.0.0`, not `localhost`** (e.g. `next dev -H 0.0.0.0`), or
  the published port will not reach it.
- **Use `npm run <script>` / `npx`, not the bare binary**, so `node_modules/.bin` is on
  `PATH`.

## Setup and diagnostics

```sh
harv doctor                # check runtime, image, config, and platform
harv scaffold              # write a starter .harvey.yaml
harv version               # harvey version + detected runtime
```

## Two modes: your dev box vs. a sandbox for agents

People want their own dev environment to be permissive (their tools, their files) but an
autonomous AI agent to be boxed in. harvey runs in exactly those two profiles:

| | `harv` / `harv <cmd>` (dev) | `harv sandbox <cmd>` (locked down) |
|---|---|---|
| Mounts | full `$HOME`: projects, dotfiles, caches | only the workspace (`$PWD`) |
| Network | on | off (`--network none`) |
| Root filesystem | writable | read-only + tmpfs `/tmp` |
| Capabilities | default | dropped (`--cap-drop ALL`) |
| Environment | your config | only what you list in `sandbox.env` |
| For | you, working | AI agents / untrusted code |

The deep dive on the sandbox profile, its threat model, per-runtime support, and
per-agent recipes lives in [sandbox.md](sandbox.md).
