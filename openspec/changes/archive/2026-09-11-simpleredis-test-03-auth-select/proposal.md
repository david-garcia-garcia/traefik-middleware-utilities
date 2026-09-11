## Why

SimpleRedis already closes the socket when AUTH or SELECT fails during dial, but those branches have zero coverage. `TestRejectedAuthIsReturned` Inits with an empty password so AUTH never runs, and `TestAuthAndSelectOncePerDial` only covers a successful handshake. A later change can drop `conn.close()`, pool an unauthenticated socket, or retry-storm a failed handshake, and CI will still look green.

## What Changes

- Make the in-process fake's AUTH and SELECT replies configurable (default `+OK` so existing success tests stay). Handshake-failure tests Init with a password and assert mapped errors, empty idle, peer hangup, and one accept (no `exec` redial).
- Prove live AUTH/SELECT failures on Redis and Dragonfly for the cases each engine supports: `SELECT 99` on the existing unpassworded engines; wrong password on sibling `--requirepass` services (`WRONGPASS` → `redis:noauth`). Keep `/redis` and `/dragonfly` no-password empty-database.
- Probe `Config` gains `Password` and `Database` (empty default). Pester adds failure whoami routes. Any Eval the harness still sends stays the existing Lua 5.1-safe KEYS-declared snippet.
- Do not change production `dial`, `replyError`, pool, or timeout logic unless a new test proves it wrong. Do not map Redis 7.4 nopass AUTH text to `redis:noauth`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_tcp-session`: AUTH or SELECT handshake failure closes the socket, does not pool it, surfaces the mapped error, and does not redial; fake plus skip-if-unset live Redis and Dragonfly proof.
- `std_go_simpleredis_resp-commands`: keep compose no-password empty-database SHALL for `/redis` and `/dragonfly`; add Pester failure routes (wrong password, `database=99`) via probe `Password`/`Database`; Eval stays Lua 5.1-safe with KEYS required.

## Impact

- `simpleredis/simpleredis_test.go` (fake handshake replies, hangup counter, handshake-failure tests, live skip-if-unset tests). Production `simpleredis.go` unchanged unless a test proves it wrong.
- `e2e/simpleredisprobe/` (`Password`, `Database`), `docker-compose.yml` (sibling `redis-auth` / `dragonfly-auth` plus failure whoami routes), `scripts/integration-tests.Tests.ps1`, `.github/workflows/ci.yml` (passworded sibling services + `SIMPLEREDIS_LIVE_*` env).
- Main specs `openspec/specs/std_go_simpleredis_tcp-session/spec.md` and `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Reclaim routes `/a` `/b` and success `/redis` `/dragonfly` stay. Windowcounter/tokenbucket unpassworded CI services stay.
