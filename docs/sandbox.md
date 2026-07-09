# Sandboxing AI agents

`harv sandbox <cmd...>` runs in a locked-down profile for untrusted code and autonomous
AI agents. It **inverts** the permissive dev default: same image (so the agent keeps the
toolchain), but none of your host.

| | dev (`harv`, `harv <cmd>`) | sandbox (`harv sandbox <cmd>`) |
|---|---|---|
| Mount | full `$HOME` + configured mounts | only the workspace (`$PWD`/`sandbox.workspace`) |
| `$HOME` inside | your real `$HOME` | ephemeral `/tmp` (tmpfs) |
| Network | on (or `network:`) | `none` (where supported) |
| Root filesystem | writable | `--read-only` + `--tmpfs /tmp` |
| Capabilities | default | `--cap-drop ALL` |
| Environment | `env`/`envFile` | only `sandbox.env`/`sandbox.envFile` |

Threat model addressed: secret exfiltration (`~/.ssh`, cloud creds, tokens), writes
outside the project, prompt-injection-driven network egress, and (on Apple `container`)
kernel-level escape via the per-container microVM.

## Configure

```yaml
sandbox:
  workspace: $PWD                 # the only rw mount (default)
  network: none                   # block egress (default)
  env:
    - ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY   # inject only what the agent needs
  memory: 4g
  cpus: 2
```

## Per-runtime support

| Control | Docker | Podman | Apple `container` |
|---|---|---|---|
| workspace-only mount, read-only, tmpfs, `--cap-drop`, `--memory`, `--cpus` | yes | yes | yes |
| `--network none` | yes | yes | **no** (warned; container keeps network) |
| `no-new-privileges` / seccomp | yes | yes | **no** (warned) |
| kernel isolation | shared host kernel | shared host kernel | **own kernel (microVM)** |

`harv` applies what the runtime supports and prints a one-line notice for anything it
can't enforce, e.g. on Apple `container`:
`sandbox on container cannot enforce: network isolation (--network none), no-new-privileges`.

## Per-agent recipes

harvey is agent-agnostic: it wraps the whole agent process; it does not read the agent's
own config. Put the agent in your image (or a `sandbox.workspace` tool), inject its key
via `sandbox.env`, and run it in its most autonomous mode, the container is the boundary:

```sh
harv sandbox 'claude --dangerously-skip-permissions'   # Claude Code, unattended
harv sandbox 'codex --full-auto'                        # Codex CLI
harv sandbox 'aider --yes'                              # Aider
```

These tools also ship their own OS-level sandboxes (Seatbelt/Landlock/seccomp); those
are complementary in-process layers. harvey is the outer container boundary and the only
one that also works under Apple `container` and across all three runtimes.
