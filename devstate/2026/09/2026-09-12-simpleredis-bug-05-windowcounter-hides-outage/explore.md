# Explore
IssueKey: 2026-09-12-simpleredis-bug-05-windowcounter-hides-outage

## Concepts

Buffered `windowcounter` (`sync_rate > 0`) admits from `redis_known + local_delta` and flushes with EVAL. Exact mode (`sync_rate == 0`) talks to Redis on every Take. The live spec `std_go_windowcounter_sliding-take` says Redis errors propagate on Take and Peek: the library MUST NOT fail-open, fail-close, or health-gate; unreachable MUST NOT admit or deny as a silent fallback.

On DestBranch, `flushLoop` / `Sleep` / `Close` discard `flushPending`'s error (`windowcounter/limiter.go:350`, `:285`, `:308`). `windowLocked` GETs only when `localDelta == 0` (`:235-241`). A hot key with a pending delta never observes Redis, so `Take` returns `(allowed, estimate, nil)` through an outage. Exact `takeExact` already returns the Redis error. Usage packet `knowledge/devdocs/std_go_windowcounter.md` documents `sync_rate` as flush/accuracy only.

Kong research `knowledge/research/ext_kong_rate-limiting_sliding-sync/notes.md` covers `sync_rate` 0 vs >0 and the OSS EVAL flush. It does not state what Kong does when that flush fails. This library's sliding-take spec is the owner of the failure contract, not Kong.

Identity: the library never reads HTTP, client address, user, tenant, or Host. The opaque key is the caller's (`sliding-take` "Caller owns the key"). No identity owner to reuse inside `windowcounter`.

Call sites of `Take` / `Peek` / `Allow`: `windowcounter/limiter.go` `Allow` → `Take` (one production alias). All other hits are `windowcounter/limiter_test.go`, `limiter_e2e_test.go`, `limiter_yaegi_test.go`. No other package in this tree constructs `windowcounter.New` (searched `**/*.go`). Take already returns `(bool, float64, error)`.

`testFakeRedis` (`windowcounter/fake_redis_test.go`) closes the listener on cleanup only. No kill of live sockets.

## Decisions

Buffered outage is a silent gap between flushes, not a blanket fail-open. Per-instance `limit` still applies because `localDelta` keeps growing when flushes fail. The global cap becomes `limit × N` with no error.

This run ships **one** surface: retain the flush error and return it from Take and Peek on the existing error slot, bounded by a **staleness deadline of `1 × sync_rate`**. Not `LastFlushError()` / `Stale()`. Not a fourth return. Not GET-on-every-buffered-Take.

Mechanism:

- On `Limiter`, under `l.mu`: `lastFlushErr error`, `flushFailedAt time.Time`, `lastRedisOK time.Time`.
- `flushLoop`, `Sleep`, and `Close` store `flushPending`'s error instead of `_ =`. Success clears `lastFlushErr` and sets `lastRedisOK`. Seed GET success also sets `lastRedisOK`.
- Take/Peek: if `lastFlushErr != nil`, return that error on the existing third value. If it is nil and `now.Sub(lastRedisOK) >= syncRate` (k = 1), probe with **one** `flushPending` (not a GET), then return that error if it failed.
- Buffered Take still increments `localDelta` and still returns the local `allowed` / estimate so a caller that ignores the error can fail-open. Exact mode is unchanged (`false, 0, err` after the Redis call fails).
- Returning a Redis error from a failed flush or probe is error propagation, not a health-gate. A homemade stale error with no Redis contact would be a health-gate; do not invent one.
- Peek uses the same lastFlushErr / staleness path as Take (`peekCountLocked` / `peekBuffered`). Spec Peek unreachable SHALL; Take-only would leave Peek as the silent sibling.
- Wrap `parseEvalInt` conversion failures: `fmt.Errorf("%s: %w", simpleredis.RedisIssue, convErr)`.
- Tests: killable fake (close listener and live sockets). Proofs 1–4 from requirement.md. Proof 1 uses long `syncRate` so the ticker does not fire; after kill, advance `SetNowForTest` by `syncRate` (or otherwise expire staleness) so Take probes without a real tick.

Document exact vs buffered failure next to `syncRate` in `New`, README, and `knowledge/devdocs/std_go_windowcounter.md`.

## Open questions

- Q: Which of the three error surfaces does this run ship (Take error, LastFlushError/Stale poll, or staleness deadline)?
  Rank: bounded asked — Desired names pick one surface and keep Take's admit decision; 1 production alias (`Allow`) plus test files in `windowcounter/` (roots `**/*.go` for `windowcounter.New` and `.Take(` / `.Peek(` / `.Allow(`); no other package calls them)
  Decision: assumed — staleness k=1 plus return `lastFlushErr` from Take and Peek on the existing `(bool, float64, error)` slot; store `lastFlushErr` / `flushFailedAt` / `lastRedisOK`; probe with flushPending when stale; do not add LastFlushError or a fourth return; do not GET every buffered Take.
  By: explore

- Q: What is the staleness multiplier k?
  Rank: additive asked — Desired names k if the staleness surface is chosen; new fields on Limiter this change creates
  Decision: assumed — k = 1 (one missed `sync_rate` interval). `now.Sub(lastRedisOK) >= syncRate` triggers one flushPending probe.
  By: explore

- Q: Must buffered Peek with a pending delta surface the same error as Take?
  Rank: bounded asked — Affected lists `std_go_windowcounter_sliding-take` Peek unreachable SHALL; ticket names Take; sibling Peek/Take already share the buffer (`peekCountLocked` / `windowLocked`); same call-site set as Take
  Decision: assumed — Peek returns the same lastFlushErr / staleness error as Take. Do not leave Peek as a silent sibling.
  By: explore

- Q: Is a third Take return value acceptable under Yaegi and existing callers?
  Rank: additive asked — Desired lists a third return or typed error; Take already returns three values (`limiter.go:112`)
  Decision: assumed — keep `(bool, float64, error)`. Put the flush/staleness error in the existing error slot. Do not add a fourth return. Yaegi callers already unpack three values.
  By: explore

- Q: What does Kong Advanced / OSS do when a buffered flush fails?
  Rank: additive incidental — research notes do not say; not a criterion; this repo's sliding-take spec already owns error propagation
  Decision: assumed — do not clone Kong for flush-fail; follow `std_go_windowcounter_sliding-take` Redis-errors-propagate. Kong `sync_rate` remains the accuracy knob only.
  By: explore

- Q: Who already owns client identity (address, user, tenant, Host, trust hop) for this change?
  Rank: additive asked — explore identity gate; spec "Caller owns the key"
  Decision: resolved — none in this library. Callers pass an opaque key. Window counter MUST NOT reconstruct identity (`sliding-take` Caller owns the key; usage packet `_Avoid_` reading HTTP or client address).
  By: explore
