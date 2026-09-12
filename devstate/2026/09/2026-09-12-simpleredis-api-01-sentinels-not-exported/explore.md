# Explore
IssueKey: 2026-09-12-simpleredis-api-01-sentinels-not-exported

## Concepts

- **Error string const** — exported tokens (`RedisMiss`, `RedisUnreachable`, …) for display and legacy `err.Error()` matching. Stay exported.
- **Error sentinel value** — package-level `error` vars. Dest keeps them unexported (`errMiss`, …). Callers cannot `errors.Is`.
- **Pool wait vs unreachable** — `errPoolWait` and `errUnreachable` share `Error()` text `redis:unreachable` and are distinct values. `shouldRetry` / `isUnreachable` identity-compare (`err == errUnreachable`) so pool wait is not retried (`commands_exec.go:83-105`, `TestShouldRetryPoolWaitIsFalse`).
- **Miss as zero** — `windowcounter.getCount` treats `redis:miss` as counter 0 (`limiter.go:267-274`). Dest matches `err.Error() == simpleredis.RedisMiss`.
- **Yaegi clientprobe** — interpreted GOPATH package in `simpleredis/yaegi_test.go` `clientprobeSrc`. `ioError` already uses `errors.Is(os.ErrDeadlineExceeded)` under Yaegi.
- **Usage** — `knowledge/devdocs/std_go_simpleredis.md` and `std_go_windowcounter.md` tell callers to match by `Error()` text. After apply that is incomplete (predicates / `errors.Is` become the durable match). Produce that usage in implement / devdocsimpact, not here (dest text is still true).

No identity reconstruction (client address / user / tenant / Host / trust hop). No new research write: wrapping is stdlib `errors.Is`; Yaegi `errors.Is` is already in-tree; go-redis retries `ErrPoolTimeout` via `errors.Is` and this client deliberately does not.

## Decisions

- Export `ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrPoolWait`. Keep the string consts.
- `ErrPoolWait = fmt.Errorf("%w", ErrUnreachable)` so `errors.Is(ErrPoolWait, ErrUnreachable)` is true and `Error()` stays `redis:unreachable`.
- Internal unexported names become aliases of the exported values. In-package call sites stay on `errMiss` / `errUnreachable` / `errPoolWait`.
- Keep `shouldRetry` / `isUnreachable` / `isCommandTimeout` on identity. Do not rewrite them onto `errors.Is` (out of scope except to keep pool-wait unretried). After wrapping, `errPoolWait != errUnreachable` remains true, so identity still skips pool wait.
- Add `IsMiss`, `IsUnreachable`, `IsPoolWait` via `errors.Is`. Do not add `IsTimeout` / `IsNoAuth` / `IsIssue` (not asked).
- Convert only `windowcounter/limiter.go:271` to `simpleredis.IsMiss(err)`. Leave `e2e/simpleredisprobe/plugin.go:210`, package-internal `err.Error()` tests, and tokenbucket assertions.
- Tests: wrap each sentinel and assert `errors.Is` + predicate still match while `err.Error() == token` does not; pin `errors.Is(ErrPoolWait, ErrUnreachable)`, `IsPoolWait` distinction, `shouldRetry(ErrPoolWait)==false` / `shouldRetry(ErrUnreachable)==true`; Yaegi clientprobe `errors.Is` + one predicate; windowcounter miss-as-zero through wrapping via a helper getCount already owns (see Open questions + deviations).
- Spec fold: delta on existing `std_go_simpleredis_resp-commands` (exported strings + sentinels/predicates) and `std_go_windowcounter_sliding-take` / `sync-flush` (`redis:miss` as zero via `IsMiss`). Propose runs FindSpecHost.
- Bound: no other `simpleredisfixes2` files; no wrapping of `parseEvalInt` / `RedisIssue` at `limiter.go:278`.

## Open questions

- Q: Can interpreted `clientprobe` call `errors.Is` on the exported sentinel vars, or only the predicates?
  Rank: additive asked — criterion 7 names adding `errors.Is` and one predicate to `clientprobe`; existing callers keep working
  Decision: assumed — add both; `ioError` already uses `errors.Is` under Yaegi. If interpreted `errors.Is` on package vars fails, keep the predicate (the Yaegi-friendly surface) and pin that outcome in the test.
  By: explore

- Q: How does the windowcounter wrapped-miss test inject a wrapped Get error?
  Rank: additive asked — criterion 7 names a windowcounter test where Get returns a wrapped `redis:miss` and the limiter still treats the counter as zero; adding a helper this change creates
  Decision: assumed — do not add a Redis interface or a Get test hook on `Limiter` (`*simpleredis.SimpleRedis` stays). Extract the miss-as-zero classification `getCount` already owns into a small helper this change creates; the test feeds `fmt.Errorf("context: %w", simpleredis.ErrMiss)` into that helper. Fake TCP Peek on an empty store already covers unwrapped `$-1`. `SimpleRedis.Get` does not wrap yet (bug-05 out of scope).
  By: explore
