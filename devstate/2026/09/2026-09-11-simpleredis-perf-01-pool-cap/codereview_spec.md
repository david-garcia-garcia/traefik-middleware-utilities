# Spec

1. [wrong] openspec/changes/archive/2026-09-11-simpleredis-live-pool-cap/specs/std_go_simpleredis_tcp-session/spec.md — Requirement: Live cap is proven on Redis and Dragonfly — Pester SHALL observe at most `poolSize` clients on that backend (default 8); Scenario: Concurrent holds stay within poolSize on Redis / Dragonfly — THEN at most the default `poolSize` (8) established TCP clients from that plugin
   `scripts/integration-tests.Tests.ps1:154` — `Assert-SimpleRedisLiveCap` asserts `$established | Should -BeLessOrEqual 10` after counting every `:6379` ESTABLISHED in `/proc/net/tcp`, not plugin-only clients; nine or ten connections pass while the SHALL is at most 8
   Status: skipped
   Argument: /proc ESTABLISHED is all :6379 in the netns, not plugin-only; 10 is slack for compose/sidecar; CLIENT LIST is blocked during Lua hold. Tightening to 8 would flake; weakening the session SHALL to 10 would be the wrong contract. Not applied unattended.
