## Context

See proposal.md Why. Dest already has pool, decoder, and command behaviour plus helpers (`holdGetsForTest`, `setRejectMSetEX`, `writeGopathSimpleredis`, `startStallRedis`, `raceDetectorOn`). This change adds tests and `simpleredis/BUGS.md` only. Another agent owns Yaegi handshake sentinel matching and edits `yaegi_test.go`; this change must not touch that file.

FindSpecHost:

```
verdicts:
  - { deltaId: chaos-lifecycle, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session] }
  - { deltaId: decoder-fuzz, fold, spec-id: std_go_simpleredis_resp-decode, confidence: high, candidates: [std_go_simpleredis_resp-decode] }
  - { deltaId: commands-error-paths, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands] }
```

## Goals / Non-Goals

**Goals:**
- New `_test.go` files that lock the six healthy probes.
- Default `go test ./simpleredis/` stays in the low seconds; `-short` skips chaos and lifecycle stress.
- `simpleredis/BUGS.md` on dest, pointing the three real bugs at their PRs.

**Non-Goals:**
- Any non-test edit under `simpleredis/`.
- Editing `yaegi_test.go`.
- Asserting interpreted AUTH/SELECT sentinel matching.
- Peak live-socket assertions.
- Committing other packages’ `BUGS.md`.

## Decisions

- **New files over edits.** Chaos fake lives in `chaos_pool_test.go` so `fake_redis_test.go` stays merge-clean. Yaegi error paths live in `yaegi_errorpath_test.go`. Alternative: append to existing files — rejected because three other agents share this repo.
- **Short-gate only the long stresses.** Fuzz seeds, injection, concurrent MSetEX, and Yaegi error paths stay in `-short` so CI Unit and Unit race actually run them. Chaos 24 goroutines × 1s and 200 Close cycles skip under `-short`. When `raceDetectorOn`: chaos 8 × 250ms, lifecycle 40 cycles, concurrent MSetEX 4×8.
- **At-rest socket asserts only.** Comment in the chaos test why `len(idleConns)+len(inUseTurns)` and peak `open` over-count. Alternative: assert peak ≤ PoolSize — rejected; it flakes.
- **Track `simpleredis/BUGS.md`.** Ticket owns that file. Other packages’ untracked `BUGS.md` stay untracked; the delivery card tells the human.

## Risks / Trade-offs

- [Probe fails on dest] → Do not patch product source; record `skill:opd-workflow:Issues` and put the measurement on the card.
- [Yaegi tests collide with the handshake PR] → Separate file; no AUTH/SELECT matcher asserts.
- [Default suite grows] → Short-gate the two long stresses; keep the rest as seed-corpus / modest loops.
- [Race job timeout] → Lower counts when `raceDetectorOn`; CI race already passes `-short` so chaos/lifecycle skip there.

## Migration Plan

None. Tests and a docs file only. Rollback is revert of this branch.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
