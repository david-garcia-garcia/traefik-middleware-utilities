# Explore
IssueKey: 2026-09-13-windowcounter-bug-fractional-window

## Concepts

Sliding Take/Peek share `slidingAt`. Dest rejects only `windowSec := int64(window / time.Second)` when `windowSec < 1`. A 1500ms window is `windowSec == 1`, so Redis keys and TTL follow 1-second buckets while the weight line still divides by the full Duration.

```
caller window 1500ms
        │
        ▼
slidingAt: windowSec = 1   ← accepted
        │
        ├─ keys:  floor(unix / 1) * 1     (1s buckets)
        ├─ ttl:   2 * 1                   (2s)
        └─ weight: 1 - elapsed/1500ms     (Duration denom)
```

Usage packet `knowledge/devdocs/std_go_windowcounter.md` already says window length is whole seconds. Spec `std_go_windowcounter_sliding-take` already SHALL whole seconds. Dest code does not enforce the remainder.

Identity: this change does not set or reconstruct client address, user, tenant, Host, or trust hop. Callers already own the opaque key.

## Decisions

- Reject in `slidingAt` when `window < time.Second` **or** `window % time.Second != 0`. Do not truncate into `windowSec` buckets. Take and Peek both call `slidingAt` (`limiter.go`); Allow aliases Take. Two call sites enumerated in that file.
- Weight and TTL stay on `windowSec` only. Weight denom becomes `float64(windowSec)*float64(time.Second)` (or equivalent). Dest TTL is already `2 * windowSec`.
- Tests first: add `windowcounter/repro_fractional_window_test.go` `TestRepro_FractionalWindowAccepted`. Subtest A is the lock (1500ms Take must error). Confirm FAIL on dest (Take succeeds), then fix, then PASS. Keep `TestTake_SubSecondWindow`.
- Fold the reject into existing spec `std_go_windowcounter_sliding-take` (scenario: 1500ms Take errors). No new spec family.
- Bound: do not take other `windowcounter` bugs (expire retry, lock during GET, Peek/Take occupancy, BUGS.md rows besides this one).

## Open questions

- Q: What error string does a fractional window return?
  Rank: additive incidental — new remainder branch on a gate this change creates; requirement lists wording as unknown, not a named criterion
  Decision: assumed — keep `windowcounter: window must be at least one second` for `window < time.Second`. Remainder uses `windowcounter: window must be a whole number of seconds` so 1500ms is not told it is under one second. Tests assert non-nil only (`TestTake_SubSecondWindow`, repro A).
  By: explore

- Q: After the fix, does the repro still assert 1-second Redis key rollover?
  Rank: additive asked — requirement Desired 1 and caller spec name subtest A as the lock (1500ms Take must error)
  Decision: resolved — A is the post-fix contract. B is dest-fail diagnostic only; when Take errors, return and skip B (parent repro already does this). Do not keep B as a post-fix contract.
  By: explore

- Q: Does Peek need its own fractional-window test?
  Rank: additive incidental — Peek shares `slidingAt`; requirement names Take
  Decision: assumed — no dedicated Peek case. The shared gate plus Take repro and `TestTake_SubSecondWindow` cover it.
  By: explore
