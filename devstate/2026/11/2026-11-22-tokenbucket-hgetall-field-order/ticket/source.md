# Requirement (FIX (reproduced on live Redis and Dragonfly))

Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/14297 ; upstream fix PR https://github.com/traefik/traefik/pull/14300
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)

Root cause: tokenbucket/lua.go reads HGETALL by position (rl_source[2]=last, rl_source[4]=tokens). Redis/Valkey do not guarantee hash field order (hashtable encoding order depends on the per-process hash seed). When tokens comes first the bucket refills on every call and the limiter never limits (reproduced: 10 of 10 admitted with burst 5).

Binding design:

- In allowScript replace HGETALL + `#rl_source == 4` with `redis.call('hmget', key, 'last', 'tokens')`; treat the bucket as existing only when both values are truthy (HMGET returns false for a missing field on Redis; verify Dragonfly Lua 5.4 behaves the same in the live test); otherwise keep the current seed branch (tokens = burst, last = t).
- Same approach as upstream PR #14300. Update the header comment of lua.go: the script is copied from Traefik @903e8a9 and now deviates in this read, citing traefik/traefik#14297 and PR #14300. Keep the rest of the Traefik script unchanged.
- allowScriptDigest is derived from allowScript; no manual change.
- Memory store is unaffected; do not touch it.
- The repro test must pass after the fix on both engines: control_last_first, tokens_first_listpack and tokens_first_hashtable (hashtable may SKIP when the server's hash seed keeps last first; that is expected and documented in the test).
- Existing TestLive_Limiter and Yaegi live tests must keep passing (the script runs under Yaegi too).

## Tests

Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik14297_HGetAllFieldOrder (tokenbucket/traefik_issues_e2e_test.go) -> new file tokenbucket/traefik_issue14297_e2e_test.go.

Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).

Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
