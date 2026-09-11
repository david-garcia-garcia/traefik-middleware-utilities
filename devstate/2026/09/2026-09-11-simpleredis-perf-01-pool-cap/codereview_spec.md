# Spec

1. [missing] openspec/changes/simpleredis-live-pool-cap/specs/std_go_simpleredis_tcp-session/spec.md — Requirement: Live cap is proven on Redis and Dragonfly — Pester SHALL observe `redis:unreachable` when a waiter exceeds the pool wait; Scenario: Extra waiter is redis:unreachable on both engines
   `scripts/integration-tests.Tests.ps1:90` — `Assert-SimpleRedisLiveCap` requires only `live >= 1` and `live <= 8`, then asserts 502/`redis:unreachable` solely inside `if ($live -ge 8)` (`:93`). Holders may finish as 200 or 502 (`:100`). When eight sockets are not seen in the 250ms window, the extra-waiter SHALL is not observed and the It still passes.
   Status: done
   Argument: Pester requires live >= 8 then always asserts 502 redis:unreachable on the ninth waiter; ESTABLISHED count uses raw regex matches.
