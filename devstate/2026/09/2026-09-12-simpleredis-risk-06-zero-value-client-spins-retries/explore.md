# Explore
IssueKey: 2026-09-12-simpleredis-risk-06-zero-value-client-spins-retries

## Concepts

`SimpleRedis` is an exported struct. `New` is the only caller of `ensureInUseTurns`. A literal `&SimpleRedis{}` keeps `inUseTurns` nil and `closed` false.

`borrow` treats a nil `inUseTurns` the same as a closed client: it returns `errUnreachable`. `exec` retries while `shouldRetry(err)` and the client is not closed. `shouldRetry` is identity on `errUnreachable`. `errPoolWait` already uses the same `Error()` text (`redis:unreachable`) with a distinct identity so pool wait is not retried.

Zero-value `MaxRetries` is 0, which `retryLimits` maps to 3 extra retries and 8 ms / 512 ms backoff. Attempt 0 fails immediately; attempts 1–3 sleep. The programming error is reported as a transient network failure after tens of milliseconds.

In-repo consumers (`tokenbucket/redis.go`, `windowcounter/limiter.go`) inject `*simpleredis.SimpleRedis` from `New`. The tcp-session spec already requires construction via `New(Config)` and that closed-client and pool-wait `redis:unreachable` are not retried. It does not name a client that never came from `New`.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` already says call `New` before concurrent use. It does not document the current retry on a zero-value client.

This work does not set or reconstruct client address, user, tenant, Host, or trust hop. No host-owned identity owner question.

## Decisions

- Keep the `errPoolWait` pattern: a new unexported sentinel beside it, `Error()` still `redis:unreachable`, identity distinct so `shouldRetry` stays false without editing `commands_exec.go`.
- `borrow` returns that sentinel when `inUseTurns` is nil. Do not call `ensureInUseTurns` from `borrow` or anywhere except `New`.
- Leave the closed-client path on `errUnreachable` plus `isClosed()`; that is a different case.
- Proof in `simpleredis/` (same package, unexported sentinel): zero-value `Get` elapsed under default `MinRetryBackoff` (8 ms) and `shouldRetry` false; smoke every exported command on `&SimpleRedis{}` (non-empty `MGet` / `MSetEX` / `MSetEXAt` so they reach `borrow`); `Close` twice with no panic; `cap(sr.inUseTurns) == 0`.
- Spec delta on existing `std_go_simpleredis_tcp-session` (not a new family). Usage gotcha after implement: a client not from `New` fails the first command immediately.
- Bound: this finding only. No panic, no compile-time unusable zero value, no Host validation, no `tokenbucket` / `windowcounter` changes.

## Open questions

- Q: What wall-clock ceiling proves “well under one backoff interval” for zero-value Get?
  Rank: additive asked — new assertion this change creates; Desired names well under one backoff interval
  Decision: assumed — elapsed MUST be `< 8ms` (default `MinRetryBackoff`, the first sleep floor). Reproduced DestBranch Get on `&SimpleRedis{}` at 58.9175 ms. Pair with `shouldRetry(err) == false` so the identity proof does not depend on the clock.
  By: explore

- Q: Do any out-of-repo callers construct `SimpleRedis` without `New`?
  Rank: additive asked — Desired names same `Error()` text so string matchers still work
  Decision: assumed — they may exist; keep `Error()` as `redis:unreachable`; do not reshape the exported type.
  By: explore
