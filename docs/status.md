# harvey status

A snapshot of what works and what's next. `harv` is functional and verified live
against Apple `container` (macOS) and Podman (Linux).

## Working

Run modes
- interactive shell (`harv`), throwaway command (`harv <cmd>`)
- `harv exec NAME` into a running container, `harv serve` (detached + published ports)
- `harv enter [NAME]` persistent ("pet") container: name derived from the dir or given, reused/restarted across runs and image rebuilds, discarded with `harv rm`

Lifecycle & images
- `ls`/`ps`, `logs`, `rm`, `cp`
- `init`/`recreate` with `--build-arg`/`--target`/`--no-cache` and a `postBuild` hook
- default Red Hat UBI9 image (zero config); registry images auto-pull on first run;
  only `build:`-based images need `harv init`

Cross-cutting
- multi-platform `--platform`/`platform:` (amd64 emulation verified)
- `harv sandbox <cmd>` locked-down profile: workspace-only mount (no `$HOME`),
  `--network none` (sealed on both runtimes, incl. Apple `container`), read-only rootfs,
  `--cap-drop ALL`, dedicated `sandbox.env`, optional dedicated `sandbox.image`; ignores
  `runArgs` and dev driver extras by design; on Apple `container` only no-new-privileges
  is unavailable (warned)
- `doctor`, `version`/`--version`, `scaffold`, shell completion, styled help/errors
  (fang), documented exit codes (spec.md), idempotent `rm`/`init`
- `.harvey.yaml` overlay (user + nearest project file), strict decode, `$VAR`/`~`
  expansion, `HARVEY_IMAGE`/`HARVEY_RUNTIME` + flag overrides
- pluggable `Driver` (container/podman); add a runtime in one file + one line (the seam
  where a Docker driver could be revived on demand)

Quality / infra
- `go vet` + golangci-lint (v2) clean
- `scripts/test.sh`: automated end-to-end, cleans up on failure, passes both runtimes
- GitHub Actions CI (vet/build/test/lint), Dependabot, AGPL-3.0

## TODO / roadmap

- [ ] Sandbox network allowlist via a hostname-filtering forward proxy (today: `none`
      or a named network)
- [x] Apple `container` 1.0.0 failed to start a container whose stdout is `/dev/null`
      (ENODEV); fixed upstream in 1.1.0 (verified 2026-07-09). `scripts/test.sh` keeps
      its log-file routing for anyone still on 1.0.0
- [ ] Release & distribution: tagged releases, possibly a Homebrew tap
- [ ] Editor integration notes (IntelliJ / VS Code integrated terminal + tasks)
- [ ] Expand per-agent sandbox recipes in [sandbox.md](sandbox.md) as the tools evolve
