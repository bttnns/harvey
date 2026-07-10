# AGENTS.md

Drop this in your home directory (`~/AGENTS.md`) so AI agents and humans know to do
dev work in throwaway containers via [`harv`](https://github.com/bttnns/harvey),
keeping the host clean. Run toolchains and agents in the container, not on the host.

## Install harv (if the `harv` command is missing)

Build it once onto your PATH:

```sh
git clone https://github.com/bttnns/harvey.git ~/Dev/harvey
cd ~/Dev/harvey && go build -o ~/.local/bin/harv .
```

Then check it with `harv version` and `harv doctor`.

## Configure once

Copy `example.harvey.yaml` to `~/.config/harvey/config.yaml` (global) or to a project
`.harvey.yaml`, and set `image:`. With no config at all, harv uses a default base
image and pulls it on first run, no Dockerfile to write.

## Run things in the container

```sh
harv                 # interactive shell in the container, current dir
harv go test ./...   # run one command, throwaway (--rm); quote 'a && b' to chain
harv make build
```

`$HOME` is mounted at the same path, so your files and `~/.cache` persist across runs.

## Browser-facing servers

```sh
harv serve --name app -p 3000 npm run dev -- -H 0.0.0.0 -p 3000
harv ls            # running containers (alias: harv ps)
harv logs app -f   # follow logs
harv rm app        # stop and remove
```

Bind servers to `0.0.0.0` (not `localhost`), and use `npm run`/`npx` (not bare binaries).

## Sandbox AI agents

Run an autonomous or untrusted agent locked down, only the project is mounted, no
`$HOME`, no network, read-only rootfs, dropped capabilities:

```sh
harv sandbox claude
harv sandbox 'npm test'
```

More: `harv help`, and the harvey README / SPEC.
