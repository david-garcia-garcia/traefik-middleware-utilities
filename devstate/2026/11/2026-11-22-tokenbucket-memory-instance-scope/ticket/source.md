# 2026-11-22-tokenbucket-memory-instance-scope

issueHost: local
issueRef: none

## Requirement (DOCUMENT + TEST (reproduced as a contract difference; not fixable inside the library))
Upstream source of this bug (MUST be cited): Issue https://github.com/traefik/traefik/issues/13706 ; upstream PRs https://github.com/traefik/traefik/pull/13823 (open, scope option) and https://github.com/traefik/traefik/pull/13707 (closed)
Write these links into requirement.md under a heading "Upstream" stating that the existence of the defect was taken from those upstream reports, and make sure the delivery card / PR summary contains the same links in its summary text. The PR body must name the upstream issue(s) and PR(s).

Full analysis of all upstream issues: D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros\tokenbucket\TRAEFIK-UPSTREAM-ISSUES.md (read the section for this issue). You may READ that file. Do not modify that worktree.

## Design (binding: decided by the tech lead; the propose phase transcribes it into the OpenSpec change, it does not redesign)
Upstream: Traefik builds one in-memory limiter per router, so N routers give each source N budgets, while Redis keys by middleware and shares one budget. In this library each *Memory is an independent key space and Redis limiters on the same key share state; which one a middleware gets depends on how it builds and shares limiters, not on the library.
Binding design:
- No behavior change. Make the scope part of the documented contract: GoDoc on Memory/NewMemory (each value is its own key space; to share a budget across routers or reloads, share one *Memory, e.g. via reclaim keyed by middleware name) and on Redis/NewRedis (state lives in the server; any limiter on the same key shares it, so the key prefix defines the scope).
- Rewrite the repro test so it asserts the contract instead of failing: (a) two routers sharing one *Memory admit burst in total; (b) two separate *Memory values admit 2*burst (documented per-instance behavior, with a comment citing traefik/traefik#13706); (c) live: two Redis limiters on one key admit burst in total. A middleware-level fix belongs to the consumer and is out of scope; record it as a note in devstate issues + knowledge/debt.

## Tests
Reference repro tests already written and run against live engines in D:\repositories\traefik-middleware-utilities-2026-11-22-tokenbucket-traefik-upstream-repros . Use: TestTraefik13706_MemoryInstancesPerRouter (traefik_issues_test.go) and TestTraefik13706_RedisInstancesShare (traefik_issues_e2e_test.go) -> tokenbucket/traefik_issue13706_test.go and tokenbucket/traefik_issue13706_e2e_test.go.
Rules: the new test file(s) must be self-contained for this issue. Helpers the test needs live in its own file with names suffixed by the issue number (e.g. countAdmits14297, liveKey14297) so the parallel PRs never declare the same identifier; do not create shared helper files and do not copy other issues' tests. Tests must assert the behavior a correct limiter must have (no tautologies); keep the control cases. Live tests use the same gating as TestLive_Limiter (skip under -short or without the env vars) via the existing runForEachLiveEngine/waitLiveClient helpers in tokenbucket/limiter_e2e_test.go (simpleredis has its own runForEachLiveEngine/newTestRedis). Local engines already running in Docker: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret). Local gate before pushing: go vet ./... ; go test -short -count=1 ./... ; the live tests of the touched package with those env vars. CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper enabled, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
