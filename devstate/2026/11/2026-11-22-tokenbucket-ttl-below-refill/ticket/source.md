# Requirement (FIX (reproduced in Memory, Redis and Dragonfly))

Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/13957

Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue).

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)

Root cause: an idle bucket that expires (Memory delete / Redis EXPIRE) is recreated full. When ttl < refill time, a client that pauses longer than ttl gets the missing tokens for free (30 req/min, burst 30, Traefik's ttl=3s: 30 admitted after a 5 s pause instead of 2).

Binding design (tech-lead decision, do not redesign):

- NewMemory and NewRedis raise the effective TTL to a floor: refillFloor = ceil(burst/rate + maxDelay.Seconds()) whole seconds. effectiveTTL = max(callerTTL, refillFloor). Never lower the caller's TTL. Rationale: TTL only exists to reclaim idle state; at or above the floor an expired bucket is already full, so recreating it full grants nothing. Raising never changes admissions, only how long idle keys live. Rejecting short TTLs was considered and rejected because it breaks callers that pass Traefik's own formula.
- validateClock keeps its current contract (ttl >= 1s, whole seconds) on the caller's value. The floor is applied after validation and stored in clockConfig.ttl, so Memory expiry and Redis EXPIRE (ttlSeconds) keep using the same value (BUGS.md bug 4 invariant).
- Overflow: burst/rate can be enormous (tiny rate). Cap the floor so the result never overflows time.Duration (clamp to the largest whole-second time.Duration) and test that case.
- GoDoc on NewMemory/NewRedis: ttl is a minimum; the library raises it to the refill time and why (cite traefik/traefik#13957).
- Tests: the two repro tests must pass after the fix (traefik_ttl case now admits 2 in Memory and <=3 live); keep the control case. Add unit tests for the floor: short ttl raised, long ttl kept, maxDelay included, overflow clamp.

## Tests

Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik13957_MemoryIdleBucketRecreatedFull (traefik_issues_test.go) and TestTraefik13957_RedisIdleBucketRecreatedFull (traefik_issues_e2e_test.go) -> tokenbucket/traefik_issue13957_test.go and tokenbucket/traefik_issue13957_e2e_test.go.

Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis).

Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
