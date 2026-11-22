# 2026-11-22-tokenbucket-redis-unreachable

Requirement (TEST + DOC; not reproduced: the library surfaces the outage and leaves the policy to the caller).

Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/12091 (closed upstream as duplicate).
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports. The delivery card / PR summary must contain the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues (READ the section for this issue, do not edit that tree): D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md

Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign):
No behavior change. Test: Redis limiter pointed at a refusing loopback port returns errors.Is(err, simpleredis.ErrUnreachable) and allowed=false. GoDoc on Redis.Allow: on an outage it returns simpleredis.ErrUnreachable (or ErrTimeout) with allowed=false; fail-open or fail-closed is the middleware's decision (upstream Traefik answers 500 with no option, traefik/traefik#12091).

Tests:
Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik12091_RedisUnreachable (traefik_issues_test.go) -> tokenbucket/traefik_issue12091_test.go.
Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).
Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
