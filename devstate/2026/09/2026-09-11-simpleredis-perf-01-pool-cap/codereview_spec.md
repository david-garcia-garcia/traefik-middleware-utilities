# Spec

1. [wrong] `openspec/changes/archive/2026-09-11-simpleredis-live-pool-cap/specs/std_go_simpleredis_tcp-session/spec.md` — Requirement: Live cap is proven on Redis and Dragonfly / Scenario: Concurrent holds stay within poolSize on Redis (and the parallel Dragonfly scenario) — "THEN … at most the default `poolSize` (8) established TCP clients from that plugin"
   `scripts/integration-tests.Tests.ps1:851` — `Assert-SimpleRedisLiveCap` asserts `$established | Should -BeLessOrEqual 10` and counts every ESTABLISHED socket to local port 6379 in the netns dump, not clients attributed to the Traefik plugin
   Status: skipped
   Argument: /proc ESTABLISHED is all :6379 in the netns, not plugin-only; 10 is slack for compose/sidecar; CLIENT LIST is blocked during Lua hold. Tightening to 8 would flake; weakening the session SHALL to 10 would be the wrong contract. Not applied unattended.
