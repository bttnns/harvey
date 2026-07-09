# harvey status

A snapshot of what works and what's next. `harv` is functional and verified live
against Apple `container`, Podman, and Docker on macOS.

## Working

Run modes
- interactive shell (`harv`), throwaway command (`harv <cmd>`)
- `harv exec NAME` into a running container, `harv serve` (detached + published ports)

Lifecycle & images
- `ls`/`ps`, `logs`, `rm`, `cp`
- `init`/`recreate` with `--build-arg`/`--target`/`--no-cache` and a `postBuild` hook
- default Red Hat UBI9 image (zero config); registry images auto-pull on first run;
  only `build:`-based images need `harv init`

Cross-cutting
- multi-platform `--platform`/`platform:` (amd64 emulation verified)
- `harv sandbox <cmd>` locked-down profile: workspace-only mount (no `$HOME`),
  `--network none`, read-only rootfs, `--cap-drop ALL`, dedicated `sandbox.env`;
  degrades with a warning on Apple `container`
- `doctor`, `version`/`--version`, `scaffold`, shell completion
- `.harvey.yaml` overlay (user + nearest project file), strict decode, `$VAR`/`~`
  expansion, `HARVEY_IMAGE`/`HARVEY_RUNTIME` + flag overrides
- pluggable `Driver` (container/podman/docker); add a runtime in one file + one line

Quality / infra
- `go vet` + golangci-lint (v2) clean
- `scripts/test.sh`: automated end-to-end, cleans up on failure, passes all 3 runtimes
- GitHub Actions CI (vet/build/test/lint + docker e2e), Dependabot, AGPL-3.0

## TODO / roadmap

- [ ] Unit tests: config merge/expand, port normalization, dev vs sandbox arg assembly
- [ ] Sandbox network allowlist via a hostname-filtering forward proxy (today: `none`
      or a named network)
- [ ] Optional separate `sandbox.image` (run agents in a leaner/different image)
- [ ] Release & distribution: tagged releases, `go install` docs, possibly a Homebrew tap
- [ ] Editor integration notes (IntelliJ / VS Code integrated terminal + tasks)
- [ ] Expand per-agent sandbox recipes in [sandbox.md](sandbox.md) as the tools evolve
