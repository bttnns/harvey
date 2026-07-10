# Sandboxing AI agents

`harv sandbox <cmd...>` runs in a locked-down profile for untrusted code and autonomous
AI agents. It **inverts** the permissive dev default: same image (so the agent keeps the
toolchain), but none of your host.

| | dev (`harv`, `harv <cmd>`) | sandbox (`harv sandbox <cmd>`) |
|---|---|---|
| Mount | full `$HOME` + configured mounts | only the workspace (`$PWD`/`sandbox.workspace`) |
| `$HOME` inside | your real `$HOME` | ephemeral `/tmp` (tmpfs) |
| Network | on (or `network:`) | `none` (sealed on both runtimes) |
| Root filesystem | writable | `--read-only` + `--tmpfs /tmp` |
| Capabilities | default | `--cap-drop ALL` |
| Environment | `env`/`envFile` | only `sandbox.env`/`sandbox.envFile` |
| `runArgs` / dev driver extras | applied | **ignored by design** |

A sandbox run never inherits the dev-mode driver extras (on Podman that would turn
SELinux off with `--security-opt label=disable`) or the project's `runArgs` escape hatch
(which could re-mount `$HOME` or re-enable the network). Both are dropped so nothing you
set for your trusted dev box can silently weaken the fence; the sandbox gets only its own
lockdown flags plus the minimal per-runtime opts it needs (on Podman, just
`--userns=keep-id` so the container user can write the mounted workspace).

Threat model addressed: secret exfiltration (`~/.ssh`, cloud creds, tokens), writes
outside the project, prompt-injection-driven network egress (sealed on both runtimes),
and kernel-level escape (on Apple `container`, contained by the per-container microVM).

## Configure

```yaml
sandbox:
  image: my-agent-sandbox         # optional: a leaner/locked-down image; unset uses image:
  workspace: $PWD                 # the only rw mount (default)
  network: none                   # block egress (default)
  env:
    - ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY   # inject only what the agent needs
  memory: 4g
  cpus: 2
```

## Per-runtime support

| Control | Podman | Apple `container` |
|---|---|---|
| workspace-only mount, read-only, tmpfs, `--cap-drop`, `--memory`, `--cpus` | yes | yes |
| `--network none` | yes | **yes** (undocumented upstream, verified live; e2e egress probe guards it) |
| `no-new-privileges` / seccomp | yes | **no** (warned) |
| kernel isolation | shared host kernel | **own kernel (microVM)** |

Apple's `container` accepts `--network none` even though its docs describe only
`--network <name>`; live checks (2026-07-09, container 1.0.0 and 1.1.0) confirmed the
sealed container sees only loopback while raw egress and DNS both fail, and the e2e suite
carries an egress probe to catch an upstream regression.

The isolation on Apple `container` has a memory cost worth naming: it runs one Linux
microVM per container, so N parallel `harv` runs pay N kernel overheads and peak memory
is higher than single-shared-VM tools (OrbStack-style) that amortize one Linux kernel
across every container. On Podman (Linux) there is no such overhead, since containers
share the host kernel.

`harv` applies what the runtime supports and prints a one-line notice for anything it
can't enforce, e.g. on Apple `container`:
`sandbox on container cannot enforce: no-new-privileges`.

## Per-agent recipes

harvey is agent-agnostic: it wraps the whole agent process; it does not read the agent's
own config. Put the agent in your image, inject one key via `sandbox.env`, run it in its
most autonomous mode, and let the container be the boundary. Four common agents:

| Agent | Sandbox invocation | Autonomy flag |
|---|---|---|
| Claude Code | `harv sandbox 'claude --dangerously-skip-permissions'` | `--dangerously-skip-permissions` (= `--permission-mode bypassPermissions`) |
| Codex CLI | `harv sandbox 'codex --full-auto'` | `--full-auto` (workspace-write + auto-approve) |
| Aider | `harv sandbox 'aider --yes-always'` | `--yes-always` |
| OpenCode | `harv sandbox 'opencode run "<prompt>"'` | no flag; autonomy set in `opencode.json` |

Headless / CI variants: `claude -p '<prompt>' --dangerously-skip-permissions`,
`codex exec --full-auto '<prompt>'`, `aider --yes-always --message '<prompt>' <files>`,
`opencode run '<prompt>'`. Flag names drift between releases, verify with `<agent> --help`.

Codex note: `--full-auto` also turns on Codex's own workspace-write sandbox (Landlock +
seccomp on Linux), which can fail to initialize under `--cap-drop ALL` /
`no-new-privileges`. If it errors, run `codex exec --dangerously-bypass-approvals-and-sandbox`
and let harvey be the only fence. Verify with `codex --help`.

OpenCode note: OpenCode has no per-run bypass flag; its permissions live in
`opencode.json`. The sandbox has no `$HOME` (HOME=/tmp, wiped each run), so put a
project-level `opencode.json` in the workspace (it is mounted rw) rather than in
`~/.config/opencode/`, and set the permissions you want to `allow` there.

### Auth

Inject exactly one credential per agent through `sandbox.env` (a literal or a `$VAR`
expanded from the host at launch) or `sandbox.envFile` (a host file path). Only the
listed vars cross the boundary; your shell env does not.

| Agent | Key |
|---|---|
| Claude Code | `ANTHROPIC_API_KEY` |
| Codex CLI | `OPENAI_API_KEY` |
| Aider | provider key, e.g. `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` |
| OpenCode | provider key named in `opencode.json`, e.g. `ANTHROPIC_API_KEY` |

Tradeoff, stated plainly: the agent runs as the process holding the key and can read its
own environment, so any key you inject is visible to the agent, and to anything that
prompt-injects it. Inject the least-privileged key (a scoped, low-quota, revocable one)
and nothing else. Never hand a sandbox your whole shell env. A key passed in
`sandbox.env` lives only inside the throwaway container and dies with it; `sandbox.envFile`
points at a host file that persists on the host (only the listed file crosses).

### Network reality

The default is `sandbox.network: none`, sealed on both runtimes (`internal/runtime/*.go`
`SandboxArgs`). That blocks all egress, including the agent's own model API. An agent that
talks to a hosted LLM cannot run under the full seal: it has no route to
`api.anthropic.com` / `api.openai.com`. `--network none` is offline-only, useful for a
purely local-tools run, not for a cloud-backed agent.

To let the agent reach its API, set `sandbox.network` to a real network; harvey passes the
value straight through as `--network <value>`:

```yaml
sandbox:
  network: bridge          # Podman: give egress so the agent reaches its API
  env:
    - ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY   # one scoped key, nothing else
```

- Podman: `bridge`, a named network, or the rootless default (`pasta` / `slirp4netns`).
  Confirm the name with `podman network ls`.
- Apple `container`: `default`, or a network made with `container network create`.
  Confirm with `container network ls`.

With egress on, every other lockdown still holds: workspace-only mount, ephemeral `$HOME`,
read-only rootfs, `--cap-drop ALL`, kernel isolation (Apple), and only the one key you
injected. What that stops: host filesystem access, host secrets (`~/.ssh`, cloud creds),
writes outside the workspace, and privilege escalation. What it does NOT stop: with egress
open a prompt-injected agent can exfiltrate anything it can read (the workspace and the
injected key) to any host, and pull down and run arbitrary code. Only `--network none`
prevents that, and it also cuts off the agent's API. There is no partial egress today: a
scoped egress gate (allowlist the API endpoint, block the rest) is on the roadmap but is
NOT shipped, do not rely on it.

### Image requirements

The agent binary must already exist in the image; harvey installs nothing. Add it to your
Containerfile (`~/.config/dotfiles/Containerfile`):

```dockerfile
# Claude Code + Codex CLI (need node/npm)
RUN npm install -g @anthropic-ai/claude-code @openai/codex

# Aider (needs python/pip)
RUN python -m pip install --no-cache-dir aider-chat

# OpenCode (verify the package name with the OpenCode install docs)
RUN npm install -g opencode-ai
```

Run the sandbox as a non-root user: Claude Code refuses to start under root/sudo. The
Podman sandbox already maps your host uid via `--userns=keep-id`; on Apple `container`
make sure the image's default user is not root.

These tools also ship their own OS-level sandboxes (Seatbelt/Landlock/seccomp); those
are complementary in-process layers. harvey is the outer container boundary and the only
one that also works under Apple `container` and across both runtimes.
