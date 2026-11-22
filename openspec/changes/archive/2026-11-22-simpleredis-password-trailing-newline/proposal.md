## Why

A Kubernetes Secret value that ends in a newline is a different Redis password. When that string is stored in `Config.Pass`, AUTH fails with WRONGPASS even though the operator believes the password is correct. The existence of this defect was taken from https://github.com/traefik/traefik/issues/12078 (cause identified in its comments: Secret values ending in a newline). This client already sends `Pass` byte for byte; this change documents and tests that contract so the same Secret mistake fails the same way here.

## What Changes

- GoDoc on `Config.Pass`: sent to AUTH byte for byte, no trimming; a trailing newline from a Secret or file makes AUTH fail (cite https://github.com/traefik/traefik/issues/12078).
- Copy as-is `TestTraefik12078_PasswordWithTrailingNewline` into `simpleredis/traefik_issue12078_e2e_test.go`. On the requirepass engines (`SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH`, password `"secret"`): the exact password authenticates, `"secret"` plus a trailing newline fails with `redis:noauth`.
- No AUTH behavior change. Do not trim `Pass` or `User` (trimming a password would be a silent security-relevant transform).

No **BREAKING** change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None.

Live AUTH/WRONGPASS already lives on `std_go_simpleredis_tcp-session` and `std_go_simpleredis_live-e2e`. Neither promises literal password bytes nor cites #12078. This ticket is TEST + DOC and does not add a spec delta. `.openspec.yaml` sets `skip_specs: true`.

## Impact

- `simpleredis/config.go`: `Config.Pass` GoDoc only.
- `simpleredis/traefik_issue12078_e2e_test.go`: new self-contained live file; reuses `runForEachLiveEngine` / `newTestRedis`. Do not create shared helper files. Do not copy other issues' tests.
- Usage packet `knowledge/devdocs/std_go_simpleredis.md` unchanged (named DOC surface is the GoDoc).
- `New`, `authArgs`, `applyDefaults`, `tokenbucket/`, `windowcounter/`, and `e2e/simpleredisprobe` unchanged.
