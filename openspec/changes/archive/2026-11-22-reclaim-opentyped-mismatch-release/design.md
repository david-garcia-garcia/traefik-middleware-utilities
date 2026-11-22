## Context

See proposal.md and `devstate/explore.md`. Options: release after the assert, check the type with `Peek` before binding, or a hybrid. `Peek` cannot cover the first call (the type exists only after `create`), so a release path is needed anyway; the hybrid only adds a second branch. `dropWhenDone` discards the `stop` that `context.AfterFunc` returns (`table.go:558`), so a naive `drop` on mismatch would later be followed by a second `drop` from the watcher.

## Goals / Non-Goals

**Goals:**

- A mismatched `OpenTyped` leaves holder count and lifecycle exactly as if it had not bound.
- Exactly one `drop` per bind on every path (release, AfterFunc, nil-Done watch).
- Yaegi-safe: no new generic forms; `OpenTyped` stays a call expression.

**Non-Goals:**

- Undoing `create` or refusing to store a value of an unexpected type.
- Changing error text, signatures, or other failure paths of `OpenWithHooks`.

## Decisions

1. **Release after the assert, via a lease.** Add an unexported `bindLease` (`key`, `incarnation *slot`, `stop func() bool`, `dropped bool`). The internal open path (shared by `OpenWithHooks` and `OpenTyped`) returns `(value any, lease *bindLease, err error)`. `OpenWithHooks` returns `value, err`. `OpenTyped` on a type mismatch calls `t.releaseLease(lease)` and returns the zero `T` and the existing error. Alternative: Peek first — rejected, does not cover the creating call. Alternative: hybrid — rejected, two paths for one effect.

2. **Exactly-once drop under `t.mu`.** `dropOnce(lease)`: under `t.mu`, if `lease.dropped` return; set it; release the lock; then `t.drop(key, incarnation)`. The `AfterFunc` callback and `watch` call `dropOnce` instead of `drop`. `releaseLease` calls `lease.stop()` when non-nil (best effort: an AfterFunc that already started still hits the guard), then `dropOnce`. A mutex-guarded bool, not `sync/atomic`, to stay on symbols already used in the package under Yaegi.

3. **nil-Done holders.** `watch` keeps polling until the incarnation finishes, as today; after a release it finds `dropped` set and does not drop again. With no other holder, the release's drop ends the incarnation, which closes `finished` and lets the watcher exit.

4. **Already-done context at bind.** Unchanged: `finishBind` drops immediately and returns the context error before `OpenTyped` asserts, so no lease is returned.

5. **Tests.** `reclaim/zzz_opentyped_mismatch_release_test.go`, helpers suffixed `MismatchRelease`: (a) grace 0, `create` returns a string, `OpenTyped[int]` fails → Close runs once and the key is unmapped while the caller's context is still live; (b) 100 failed calls on a key that another live holder keeps → holder count equals 1 (white-box) and Close has not run; cancelling the good holder then runs Close once; (c) after a mismatch release, cancelling the failed caller's context does not drop a second time (holder count never negative, Close exactly once); (d) nil-Done context (`context.Background()`) mismatch → released, no goroutine left behind once the incarnation ends; (e) successful `OpenTyped` and the existing mismatch error test unchanged. Run the Yaegi `OpenTyped` probe unchanged.

## Risks / Trade-offs

- [One small allocation per bind for the lease] → bind is off the request path (plugin construction); negligible.
- [Callback captures change in `dropWhenDone`] → covered by existing table tests plus (c).

## Migration Plan

Internal change; no API change. Rollback is revert.

## Open Questions

None.
