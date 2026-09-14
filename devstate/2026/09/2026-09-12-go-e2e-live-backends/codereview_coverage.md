# Test coverage

**Ticket job (source: requirement.md / proposal Why):** Split unit `go test -short` from a dedicated Go E2E job and expand compiled (plus Yaegi) tests that table-drive live Redis and Dragonfly for SimpleRedis verbs, pool/session paths, and limiter scenarios.

Expanded live scenarios (`TestLive_Commands`, `TestLive_Eval`, `TestLive_MSetEX`, pool/AUTH/SELECT e2e, limiter refund/peek/expire, Yaegi `*e2e_test.go`) assert values, deny/miss, TTL, and `redis:unreachable` / `redis:noauth`; reverting those bodies would fail CI e2e. The CI job split in `.github/workflows/ci.yml` is not asserted by an in-repo test (expected for workflow config).

1. [hard] Edge case untested — `simpleredis/simpleredis_e2e_test.go:29-30` (same branch in `windowcounter/limiter_e2e_test.go:29-30`, `tokenbucket/limiter_e2e_test.go:28-29`) — `liveEngineAddrs` `t.Fatal`s when exactly one of the two LIVE env vars is set; no test calls the helper with only Redis or only Dragonfly set (e2e always sets both; `-short` skips earlier)
   → Add a small `TestLiveEngineAddrs` (or table subtest) that sets one env var, clears the other, and expects failure without dialing an engine
   Status: done
   Argument: `lookupLiveEngineAddrs` plus `TestLookupLiveEngineAddrs` in simpleredis, windowcounter, tokenbucket (runs under `-short`).
