## Why

`readReply` / `readLine` poison paths (unknown type, missing CR, empty line, bad `*` count including `*-1`, truncated elements, bad element type), `Get` / `parseIntegerReply` arity checks, and `exec` retry-borrow failure are unexercised. Dest only has nested-array and garbage-integer cases, and neither asserts a dirty socket leaves the idle pool. A proxy or HTTP-shaped reply on a Redis port can then be reused as if it were a live session.

## What Changes

- Compiled fake-server table over canned malformed RESP (unknown type including HTTP-shaped, missing CR, empty line, unparseable `*` count, `*-1`, bad element type). Truncated array/bulk uses a write-then-close helper so the case is I/O, not the 1s `ioTimeout`. Every dirty row asserts the expected error **and** `len(idle) == 0`. Fold nested-array into that table.
- Second table for `Get` / `parseIntegerReply` arity (`*0` / `*2`): assert `redis:issue?`; do **not** require `idle==0` (decode was clean).
- Dedicated one-accept helper for `exec` retry-borrow fail after a dirty reused conn: expect `redis:unreachable` and `idle==0`.
- Keep `*-1` as `redis:issue?` with `clean=false`. Comment at the `count < 0` check: legal RESP2 nil array (BLPOP timeout, EXEC abort); this client has no verb that receives it, so it is not `redis:miss`. Do not map it like `$-1`.
- Keep live Traefik happy-path on Redis and Dragonfly (`/redis`, `/dragonfly`). Extend the probe + Pester with a Get-miss header on **both** routes. Eval stays `kongIncrbyExpireatScript` (KEYS, Lua 5.1-safe).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: dirty decode is `redis:issue?` or I/O and MUST NOT re-enter the idle pool; RESP2 null array `*-1` is `redis:issue?` not `redis:miss`; `Get` / integer verbs reject wrong arity as `redis:issue?` without destroying a clean conn; Traefik e2e on both engines includes Get-miss.

## Impact

- `simpleredis/simpleredis_test.go` (malformed table, truncated helper, arity table, retry-borrow helper).
- `simpleredis/simpleredis.go` — comment at the `count < 0` check only; no `*-1` mapping change.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1` — Get-miss header on `/redis` and `/dragonfly`. Compose engines stay `redis:7-alpine` and `dragonfly:v1.40.2`.
- Main spec `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
