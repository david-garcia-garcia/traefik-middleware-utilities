## Context

See proposal.md Why. `New` copies `cfg.Pass` to `sr.pass` with no `Trim`. `authArgs` sends `[]byte(sr.pass)` as the AUTH bulk. `applyDefaults` does not touch `Pass`. GoDoc on `Config.Pass` today says AUTH runs when non-empty; it does not say byte for byte or no trim. Live WRONGPASS on `std_go_simpleredis_live-e2e` and handshake AUTH failure on `std_go_simpleredis_tcp-session` already map WRONGPASS to `redis:noauth`; they do not name a trailing newline. Skip_specs: no delta.

## Goals / Non-Goals

**Goals:**
- Document that AUTH receives `Pass` as stored, including a trailing newline from a Secret or file.
- Pin that contract on the requirepass engines with the copied live test (control `"secret"` authenticates; `"secret\n"` is `redis:noauth`).

**Non-Goals:**
- Changing AUTH, `New`, `authArgs`, or `applyDefaults`.
- Adding a catalog scenario or spec id.
- Covering `\r\n` or other whitespace.
- A fake-TCP `"secret\n"` row in `TestAuthArgumentForms`.
- GoDoc on `Config.User`.
- A Gotcha on `knowledge/devdocs/std_go_simpleredis.md`.
- Porting Traefik / go-redis Secret trimming.

## Decisions

- **No trim.** Alternative: strip a trailing newline (or all whitespace) before AUTH so a Kubernetes Secret "just works". Rejected: that is a silent security-relevant transform of a credential; the binding forbids it. A Secret that ends in `\n` is a different password; AUTH must fail with `redis:noauth`.
- **GoDoc on `Config.Pass` only.** State: sent to AUTH byte for byte, no trimming; a trailing newline from a Secret or file makes AUTH fail; cite https://github.com/traefik/traefik/issues/12078. Alternative: also edit the usage packet — rejected; named DOC surface is the field GoDoc.
- **Copy the live test as-is.** Source (read-only): `simpleredis/traefik_issue12078_e2e_test.go` in the tokenbucket-traefik-upstream-repros worktree, function `TestTraefik12078_PasswordWithTrailingNewline`. Destination: the same path in this worktree. Reuse `runForEachLiveEngine` (`SIMPLEREDIS_LIVE_REDIS_AUTH`, `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH`) and `newTestRedis`. Do not create shared helper files. Do not copy other issues' tests. Any helper this file needs must be suffixed `12078`.
- **No spec delta.** Live catalog already owns AUTH form and WRONGPASS → `redis:noauth`. This ticket does not add a live promise. `skip_specs: true`.

## Risks / Trade-offs

- [Operators still hit WRONGPASS from a Secret newline] → Documented and tested, not "fixed" by trim. Cite https://github.com/traefik/traefik/issues/12078 so the PR summary names the upstream issue.
- [CI Unit `-short` never runs the new test] → Expected: the gate is `runForEachLiveEngine` (skip under `-short` or unset AUTH addrs). The control and newline cases run on `e2e-redis` / `e2e-dragonfly`.
- [Someone later "fixes" AUTH by trimming] → The live control + `"secret\n"` → `redis:noauth` pair fails that change.

## Migration Plan

None. GoDoc and one live test file. Rollback is revert of this branch.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
