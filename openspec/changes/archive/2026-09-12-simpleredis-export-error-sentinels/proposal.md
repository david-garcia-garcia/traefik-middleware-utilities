## Why

SimpleRedis exports error *strings* but keeps every error *value* private, so callers can only match `err.Error() == token`. Window-counter miss-as-zero depends on that compare. The first `%w` wrap silently turns a miss into a hard error and a limiter deny. Exporting sentinels unblocks wrapping without breaking consumers.

## What Changes

- Export sentinel values (`ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrPoolWait`) and keep the string consts for display and legacy text matching.
- `ErrPoolWait` wraps `ErrUnreachable` so `errors.Is` matches the broad condition while `IsPoolWait` still distinguishes pool saturation.
- Add `IsMiss`, `IsUnreachable`, `IsPoolWait`. Convert `windowcounter` `getCount` to `simpleredis.IsMiss`.
- Keep `shouldRetry` / `isUnreachable` on identity so `shouldRetry(ErrPoolWait)` stays false.
- Tests: wrap each sentinel; pin pool-wait vs unreachable retry; windowcounter miss-as-zero on a wrapped `ErrMiss` via the classification helper getCount owns; Yaegi `clientprobe` `errors.Is` plus one predicate.
- Not **BREAKING**: existing `Error()` text and string consts stay. Do not convert the e2e probe or other `err.Error()` matches. Do not wrap `parseEvalInt` / `RedisIssue`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: export sentinel values and predicates; callers MAY `errors.Is` through wrapping; string consts remain for display and legacy text matching. Yaegi `clientprobe` SHALL observe `errors.Is` and one predicate.
- `std_go_simpleredis_tcp-session`: pool-wait MUST stay unretried after `ErrPoolWait` wraps `ErrUnreachable` (`shouldRetry` identity, or check `ErrPoolWait` first).
- `std_go_windowcounter_sync-flush`: previous-window and exact GET miss SHALL use `simpleredis.IsMiss` (works through wrapping), not `err.Error() == RedisMiss`.

## Impact

- `simpleredis/simpleredis.go` (exported vars + predicates).
- `simpleredis/commands_exec.go` (aliases only unless identity needs a pool-wait-first guard).
- `simpleredis` wrapping / `shouldRetry` tests and `yaegi_test.go` `clientprobe`.
- `windowcounter/limiter.go` (`getCount`) and a wrapped-miss helper test.
- Usage packets `knowledge/devdocs/std_go_simpleredis.md` and `std_go_windowcounter.md` after apply (match via `IsMiss` / `errors.Is`).
- Main specs of the three capabilities after archive.
