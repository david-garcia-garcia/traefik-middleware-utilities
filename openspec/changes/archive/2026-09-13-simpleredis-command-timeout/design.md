## Context

Dest `do` called `SetDeadline` once for the write plus the whole reply, using `clampTimeout(ctx, IOTimeout)`. The overall budget already existed as `bindCommandDeadline` plus `watchConnClose`, so `IOTimeout` was being spent twice: once shaping the budget, once capping the socket. Caller vs library timeout ownership is `ioOrContext` / `libraryTimeout`. Proceed policies: `devstate/explore.md`.

Two earlier revisions of this change were rejected on human review. They are recorded under Rejected Alternatives below, because both are the kind of thing a later reader would otherwise re-propose.

## Goals / Non-Goals

**Goals:**
- One bound on a command socket: the remaining command budget.
- Every exported timeout knob bounds something individually, so nobody needs a formula to read the latency ceiling.
- A steadily streaming multi-megabyte bulk returns intact at zero Config.
- A drip peer cannot hold an in-use turn past the budget.
- `MaxRetries` cannot inflate the latency ceiling.
- A sane default I/O share: 250 ms rather than 100 ms, before that share is folded away.
- Existing caller-deadline vs `redis:timeout` tests stay green.

**Non-Goals:**
- Changing `readBulk` allocation (sibling BUG-4).
- Shrinking `maxBulkLength` or adding a bandwidth model (deviation).
- A deprecated `IOTimeout` alias.
- Matching go-redis's knob surface (`ReadTimeout` / `WriteTimeout` and its `-1` / `-2` sentinels).
- Other packages, `syscall`, `net.Error` asserts.

## Decisions

1. **`do` stamps `SetDeadline` from the remaining command budget.** `commandBudgetLeft(ctx)` is `time.Until(ctx.Deadline())`, which is exactly what `exec` bound at entry. One line replaces the old clamp.

2. **`Config.CommandTimeout` replaces `Config.IOTimeout`; the product is gone.** `bindCommandDeadline` binds `now + CommandTimeout`. This is the whole point of the change rather than a rename: after decision 1, `IOTimeout` bounded no operation. It appeared only as a multiplicand and as a `commandBudgetLeft` fallback that `exec` never reaches, since `bindCommandDeadline` always returns a ctx carrying a deadline. Keeping it would have meant exporting a knob whose documentation had to be a formula.

3. **`MaxRetries` leaves the deadline and keeps only the attempt count.** `bindCommandDeadline` no longer takes it as a parameter. Retries end at whichever comes first, count or budget. Dest coupled them, so `MaxRetries: 3` quietly tripled the ceiling; attempt count and latency ceiling are separate operator concerns and now read separately.

4. **`DialTimeout` stays, unchanged and un-folded.** It has real individual meaning: `net.Dialer{Timeout: sr.DialTimeout()}` caps one dial attempt, and `DialContext` clamps it against the command budget too, so a dial attempt is bounded by the smaller of the two. Collapsing it into `CommandTimeout` would remove the only way to fail fast on an unresponsive host while still allowing a long read, which is exactly the WAN case.

5. **`defaultCommandTimeout` is 900 ms, not 1 s.** This change also raises the default I/O share 100 ms → 250 ms, which takes the derived budget from dest's 600 ms to 900 ms. 900 ms is then `(1+1)*(200ms+250ms)` to the millisecond, so the collapse itself moves no instant: it renames the ceiling the branch already had, and no existing budget test changes meaning. 1 s reads rounder and was considered, but it would widen the ceiling a second time as a side effect of an API-coherence change, which is a behavior change smuggled into a refactor. An operator who wants 1 s writes 1 s.

   Be precise about which step owns the widening. Against `origin/master` the zero-Config ceiling goes 600 ms → 900 ms, and that is entirely the default raise; the knob collapse contributes 0 ms. Do not read "900 ms preserves the worst case" as a claim about dest.

6. **`commandBudgetLeft` becomes a method and loses `fallback`.** A single knob means the only sensible bound for a deadline-free ctx is `CommandTimeout` itself, so the caller cannot pass anything else and the parameter is noise. It is not dead: `simpleredis_e2e_test.go` calls `do` directly with `context.Background()`, and that path must still stamp a deadline.

7. **`ioOrContext` loses `ioBound` and `ioTimeout`.** It previously compared them to tell "socket deadline was really the ctx deadline" from "socket deadline was `IOTimeout`". The socket deadline now always *is* the ctx deadline when one exists, so `ctx.Deadline()` presence is the whole test. `libraryTimeout` still maps library budget to `redis:timeout` and a sooner caller deadline to `Err()`, so `TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout` keeps its answer.

8. **Leave `maxBulkLength` at `64 << 20`.** With the budget as the only bound, no knob is a size bound. The budget plus `watchConnClose` is the wall-time cap.

9. **Tests keep the `bug3` helper prefix** so sibling branches' fakes do not collide. The silent-peer test asserts elapsed is both `>= CommandTimeout/2` and `<= CommandTimeout + slack`; the lower bound is what fails if the socket deadline ever drifts back to a per-operation cap smaller than the budget.

## Rejected Alternatives

1. **`stallConn`: refresh `SetReadDeadline` per kernel `Read`, making `IOTimeout` a stall bound.** This was the first shipped revision of this PR. It kept both knobs and gave `IOTimeout` a genuine individual meaning (quiet time between kernel reads) while the budget capped total transfer. Rejected as too much machinery for the problem: it needed a full hand-written `net.Conn` implementation, because Yaegi does not promote methods from an embedded interface, so every `net.Conn` method had to be declared by hand; it added a second deadline regime to reason about on one socket; and it added a `pooledConn.stall` field. All of that to preserve a cap the command budget already provides. **Do not reintroduce it**; the spec now states the single-stamp rule as a MUST NOT.

2. **Keep `IOTimeout` as a non-applied multiplicand.** The smaller change: stamp the socket from the budget (decision 1) and leave the `Config` surface alone. Rejected because it ships a wrong public contract. A field called `IOTimeout` that bounds no I/O operation, whose only effect is one term in `(MaxRetries+1)*(DialTimeout+IOTimeout)`, cannot be documented without a formula, and its coupling to `MaxRetries` means a retry-tuning change silently moves the latency ceiling. The API was collapsed to match the mechanism instead.

## Risks / Trade-offs

- [A quiet peer costs the whole budget instead of a shorter window] → Accepted, and pinned by test. It replaces silent permanent data loss on healthy peers; lowering `CommandTimeout` lowers the ceiling directly.
- [A drip peer could pin a turn] → Mitigation: `watchConnClose` closes on ctx done; the drip test asserts elapsed ≤ budget + slack and `OverFrees()==0`.
- [Breaking rename with no alias] → Accepted: `simpleredis` is internal to this module. Production call sites are two `New` calls in `e2e/simpleredisprobe/plugin.go`; the rest are tests. The compiler finds all of them.
- [Deviation from go-redis grows] → Accepted and documented on the spec. go-redis has no whole-command bound at all (fresh socket timeout per attempt, `MaxRetries` default 3, worst case unbounded by design) and splits socket I/O into `ReadTimeout` (default 5 s) and `WriteTimeout`, each stamped once per command read/write as `min(now+timeout, ctx.Deadline())`; it also offers `ReadTimeout: -1` (no timeout beyond ctx) and `-2` (skip `SetReadDeadline`), which this session does not. Verified at pin `github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599`: `options.go` 744-758 and 697-698, `internal/pool/conn.go` 1155-1181 (`WithReader`) and 1374-1402 (`deadline`). A Traefik middleware on the request path needs a readable latency ceiling more than it needs go-redis parity.
- [Yaegi `net.Error` assert] → Mitigation: keep `errors.Is`; no conn wrapper and no type assert remain.
- [BUG-4 conflict in `readBulk`] → Mitigation: do not touch allocation.

## Migration Plan

Library behavior change plus a breaking `Config` field rename inside this module. Callers replace `IOTimeout: d` with `CommandTimeout: d` and should widen `d` if they had relied on the `(MaxRetries+1)*(DialTimeout+IOTimeout)` product being larger than `d`. Rollback is revert. No deploy key.

## Open Questions

None. Rows live on `devstate/explore.md`.
