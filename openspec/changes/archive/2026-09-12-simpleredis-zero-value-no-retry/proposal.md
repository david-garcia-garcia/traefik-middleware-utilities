## Why

A SimpleRedis built without `New` (`&SimpleRedis{}`) has a nil in-use-turn channel. `borrow` returns the same `redis:unreachable` identity that command retry treats as a transient network miss, so a programming error sleeps the default ladder (~59 ms) before failing.

## What Changes

- A client that did not come from `New` fails the first command immediately. The error still prints `redis:unreachable` so string matchers work, but its identity is distinct from dial/EOF so `shouldRetry` does not retry it (same pattern as pool-wait).
- Prove zero-value `Get` returns in under 8 ms with `redis:unreachable`; smoke every exported method on `&SimpleRedis{}` including `Close` twice with no panic; `cap(inUseTurns) == 0`.
- Do not panic on a nil semaphore, make the zero value unusable at compile time, reject empty `Host` in `New`, or call `ensureInUseTurns` outside `New`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: a client that did not come from `New` SHALL return `redis:unreachable` on the first command and MUST NOT be retried. Closed-client and pool-wait rules stay.

## Impact

- `simpleredis/simpleredis.go` (new unexported sentinel beside `errPoolWait`)
- `simpleredis/pool.go` `borrow` nil-`inUseTurns` return
- `simpleredis/commands_exec.go` unchanged if identity is distinct
- New or extended unit tests under `simpleredis/`
- Main spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` after archive
- Usage gotcha on `knowledge/devdocs/std_go_simpleredis.md`
- `tokenbucket` / `windowcounter` construction stays
