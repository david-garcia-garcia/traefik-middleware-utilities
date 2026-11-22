# simpleredis: AUTH password trailing newline (traefik/traefik#12078)

issueHost: local
issueRef: none
Requirement type: TEST + DOC (cause confirmed in this client).

## Upstream

The existence of the defect was taken from these upstream reports:

- Issue https://github.com/traefik/traefik/issues/12078 (cause identified in its comments: Kubernetes Secret values ending in a newline)

Full analysis of all upstream issues: `tokenbucket/TRAEFIK-UPSTREAM-ISSUES.md` in the read-only worktree `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros` (section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)

No behavior change (trimming a password would be a silent security-relevant transform). Test on the requirepass engines (`SIMPLEREDIS_LIVE_REDIS_AUTH` / `_DRAGONFLY_AUTH`, password `"secret"`): the exact password authenticates, `"secret"` plus a trailing newline fails with `redis:noauth`. GoDoc on `Config.Pass`: sent to AUTH byte for byte, no trimming; a trailing newline from a Secret or file makes AUTH fail (cite traefik/traefik#12078).

## Tests

Reference repro tests already written and run against live engines in `D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros`. Use: `TestTraefik12078_PasswordWithTrailingNewline` (`simpleredis/traefik_issue12078_e2e_test.go`, copy as is).

Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. `countAdmits14297`, `liveKey14297`) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as `TestLive_Limiter` (skip under `-short` or without the env vars) via the existing `runForEachLiveEngine`/`waitLiveClient` helpers in `tokenbucket/limiter_e2e_test.go` (simpleredis has its own `runForEachLiveEngine`/`newTestRedis`).

Local engines already running in Docker: `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` `SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381` `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382` (password `secret`). Local gate before pushing: `go vet ./...` ; `go test -short -count=1 ./...` ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
