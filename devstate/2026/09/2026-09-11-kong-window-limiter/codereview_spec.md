# Spec

1. [wrong] `openspec/changes/add-ratelimit-sliding-window/specs/std_go_ratelimit_sync-flush/spec.md` — Requirement: Sleep Wake Close reclaim the flush ticker — "Sleep SHALL flush pending deltas then stop the ticker"
   `ratelimit/limiter.go:201-205` stops the ticker via `stopFlushLocked()` before calling `flushPending()`.
   Status: done
   Argument: Sleep/Close flush then stop (`1979b40`).

2. [missing] `openspec/changes/add-ratelimit-sliding-window/specs/std_go_ratelimit_sliding-take/spec.md` — Requirement: Unit and interpreter tests prove Take without Traefik — "Interpreter tests that import Yaegi SHALL run the same live Take scenarios"
   `ratelimit/yaegi_test.go` live path runs only exact N-then-deny (`UntilDeny`); no interpreted buffered two-client share or sliding-at-boundary scenario.
   Status: done
   Argument: takeprobe BufferedShare + SlidingBoundary (`1979b40`).

3. [missing] `openspec/changes/add-ratelimit-sliding-window/specs/std_go_ratelimit_sync-flush/spec.md` — Requirement: Live tests run on Redis and Dragonfly in CI — "The same scenarios SHALL run interpreted (Yaegi)" and "Live tests SHALL table-drive Redis and Dragonfly addresses"
   `ratelimit/yaegi_test.go:28-46` picks one addr (`RATELIMIT_LIVE_REDIS` or fallback `RATELIMIT_LIVE_DRAGONFLY`) and does not table-drive both engines with all three live scenarios.
   Status: done
   Argument: TestYaegiLive_RedisAndDragonfly table-drives both engines (`1979b40`).
