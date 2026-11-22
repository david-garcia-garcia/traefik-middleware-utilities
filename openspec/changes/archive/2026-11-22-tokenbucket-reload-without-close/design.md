# Design

## Context

See proposal.md Why. Dest `simpleredis.New` starts no goroutine; `Close` drains idle; a quiet still-reachable client MAY keep idle sockets until Close (`std_go_simpleredis_tcp-session`). `tokenbucket.Redis` holds `*simpleredis.SimpleRedis` and has no Close. Dest `Allow` is `(Status, error)` (`tokenbucket/redis.go`). Live gate: `runForEachLiveEngine` / `waitLiveClient` in `tokenbucket/limiter_e2e_test.go`. Proceed policies: `devstate/explore.md`. Binding design is transcribed, not redesigned.

FindSpecHost:

```
verdicts:
  - { deltaId: dropped-client-gc, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_live-e2e, std_go_tokenbucket_allow, std_go_ci_test-suites] }
```

Idle-until-Close MAY stays on the existing leaf. Close-cycles on that leaf covers New/use/Close, not drop-without-Close. `std_go_tokenbucket_allow` and `std_go_ci_test-suites` already match dest Allow and live skip/run; no delta.

## Goals / Non-Goals

**Goals:**
- One own live file that simulates 50 Traefik-style reloads through a counting TCP proxy and asserts sockets 0 after GC plus `leakedGoroutines > 2` as the fail threshold.
- Transcribe dest Allow `(Status, error)` and dest live helpers.

**Non-Goals:**
- Product edits under `simpleredis/` or `tokenbucket/` non-test files.
- Copying `tokenbucket/traefik_issues_e2e_test.go` or `holdWrites`.
- Restoring three-value Allow.
- Inverted control that forces the Traefik leak.
- Asserting finalizers or `netFD`.
- Shared helper files across parallel issue PRs.

## Decisions

1. **Own file `tokenbucket/traefik_issue13618_e2e_test.go`.** Helpers `testCountingProxy13618`, `startTestCountingProxy13618`, `liveKey13618` in that package. Alternative: copy the shared reference file — rejected; it mixes other issues.

2. **Proxy counts open sockets only.** Accept loop plus bidirectional `io.Copy`; increment/decrement `open` around each accepted session. No `holdWrites`, no `forwardHeld`. Alternative: copy the reference proxy whole — rejected; write hold is #14173.

3. **Call dest `runForEachLiveEngine(t, "TOKENBUCKET_LIVE_REDIS", "TOKENBUCKET_LIVE_DRAGONFLY", ...)` then `waitLiveClient(t, addr)` so the engine is up.** Close that wait client before the reload loop so it is not in the drop set. Reloads dial `simpleredis.New` at the proxy address, not `waitLiveClient` (that helper dials the engine directly).

4. **Reload body:** `simpleredis.New(Config{Host: proxy.listener.Addr().String()})`, `NewRedis(client, 100, 50, 0, 120*time.Second)`, `status, err := limiter.Allow(context.Background(), liveKey13618(...))`. Fatal on New/NewRedis/Allow error. Do not restore `_, _, err :=`. Drop the locals without Close.

5. **Assert symptom, not mechanism.** After the loop, GC-settle until `proxy.open == 0` or 5s, then `open == 0` is an error. `leakedGoroutines := after - before - 3*int(proxy.open.Load())`; fail only when `leakedGoroutines > 2`. Implement may tighten that threshold with dest evidence; must not loosen. Peak open is logged, not asserted.

6. **No inverted control.** The test asserts a correct limiter: Allow succeeds, sockets 0, no client goroutines. Do not add a case that fails the way Traefik does.

## Risks / Trade-offs

- [Dest HEAD fails the live assertion] → Do not patch product source. Record the measurement; the ticket is test-only.
- [GC timing flakes] → Keep the reference settle loop (several GC + 50ms) and 5s deadline; do not assert peak sockets.
- [Proxy goroutines counted as leaks] → Subtract `3*open` (accept session = 3 goroutines) as the reference does; `before` is taken after the proxy is up.
- [Parallel issue PRs collide on helper names] → Suffix `13618` on every new identifier in this file.

## Migration Plan

Tests only. Rollback is revert of this branch.

## Open Questions

None. Threshold, catalog fold, Allow signature, file, proxy, and control case are on `devstate/explore.md`.
