# Spec

1. [missing] openspec/changes/archive/2026-09-11-simpleredis-live-pool-cap/specs/std_go_simpleredis_tcp-session/spec.md — Requirement: Live cap is proven on Redis and Dragonfly — Compiled tests gated on live addresses SHALL prove the same two facts on both engines (at most `poolSize` live TCP connections and pool-wait `redis:unreachable`)
   `simpleredis/live_test.go:43-53` — `runLivePoolBackend` only runs `waiterIsUnreachable` with `poolSize = 1`; there is no live-engine assertion that concurrent use keeps established sockets at or below default `poolSize` (8).
   Status: done
   Argument: live spec now assigns default poolSize concurrent proof to Pester; compiled live tests prove waiter and MAY set poolSize to avoid Lua BUSY.

2. [wrong] openspec/changes/archive/2026-09-11-simpleredis-live-pool-cap/specs/std_go_simpleredis_tcp-session/spec.md — Requirement: Live cap is proven on Redis and Dragonfly — Scenario: Concurrent holds stay within poolSize on Redis / Dragonfly — THEN at most eight established TCP clients from that plugin
   `scripts/integration-tests.Tests.ps1:153` — `Assert-SimpleRedisLiveCap` accepts `$live | Should -BeLessOrEqual 10` after counting every `:6379` ESTABLISHED in `/proc/net/tcp`, not clients from the plugin only; nine or ten connections can satisfy the cap check while the scenario SHALL is at most eight (default `poolSize`).
   Status: skipped
   Argument: /proc ESTABLISHED is all :6379 in the netns, not plugin-only; 10 is slack for compose/sidecar; CLIENT LIST is blocked during Lua hold. Not applied unattended.
