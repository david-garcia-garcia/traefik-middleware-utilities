## Why

`IdleTimeout` is documented as an idle reuse gate, and dest already sweeps the idle list on the next borrow. A hunt showed that with traffic stopped the list never shrinks: `IdleTimeout` 50 ms, 500 ms of silence, idle list 4 and server-side open sockets 4. That is dest as specified (`New` MUST NOT start a goroutine; a quiet client MAY keep sockets until `Close`). A background reaper would release those fds, but it is not a small fix in this tree. This change records that stop.

## What Changes

- No product code. No spec SHALL change. `.openspec.yaml` sets `skip_specs: true`.
- Written recommendation: keep dest (direction 3). Do not start a ticker in `New`. Do not stamp an absolute park expiry as a pretend lifetime. The request-path corpses after quiet time belong to sibling BUG-1 (stale pooled socket retry), which this change must not touch.
- Simplicity-gate stop after propose is success. Implement MUST NOT apply this change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None. `skip_specs: true`. Existing `std_go_simpleredis_tcp-session` idle-pool requirement already says `New` MUST NOT start a goroutine and a client that issues no later command MAY keep idle sockets past `IdleTimeout` until `Close`.

## Impact

- No `simpleredis/` source or test edit.
- No rewrite of `openspec/specs/std_go_simpleredis_tcp-session/spec.md` or `knowledge/devdocs/std_go_simpleredis.md` (both already state the quiet-client keep).
- Prior note `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md` stays; wiring `Close` on Traefik/`windowcounter` remains out of scope.
- Sibling branch that edits `borrow` validation is not this change.
