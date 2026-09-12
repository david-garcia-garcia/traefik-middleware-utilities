## Why

Compiled live e2e already calls every public SimpleRedis verb, but Yaegi live omits several, compiled Eval never names KEYS as its own case, and Traefik Pester dumps every verb onto one GET so a 502 cannot name the command. Isolation and the missing Traefik MSetEXAt proof need a path-dispatched probe.

## What Changes

- Add compiled live cases: Eval with KEYS/ARGV (Kong incrby+expireat shape) and MSetEXAt with a future Unix time then Get + positive TTL.
- Expand Yaegi live `LiveVerbs` to MGet, IncrBy, Expire, ExpireAt, and MSetEXAt. Fake-TCP Yaegi stays the existing subset.
- Replace the Traefik one-request header dump with path cases: `ServeHTTP` switches on the last path segment; Pester gets one `It` per case per engine (`/redis/get`, `/dragonfly/msetexat`, …). Exact `/redis` and `/dragonfly` stay health Set+Get. Handshake routers stay Set+Get 502. Recover, drop, and hold become their own paths with the same jobs they have today.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_simpleredis_live-e2e`: Compiled Eval KEYS case, future MSetEXAt TTL landing, and Yaegi live covering the full public verb set.
- `std_go_simpleredis_resp-commands`: Traefik probe is path-dispatched; Pester asserts one case per request instead of every verb header on one GET.
- `std_go_simpleredis_tcp-session`: Recover is `/redis/recover`; drop-relay is `/redis/drop`; health `/redis` is Set+Get only.

## Impact

- `simpleredis/commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`, `yaegi_test.go` `LiveVerbs`, `yaegi_e2e_test.go`.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`, `Test-Integration.ps1` health URLs (still `/redis` and `/dragonfly`).
- `openspec/specs/std_go_simpleredis_live-e2e/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`.
- Usage packets that name the Yaegi live subset or Traefik dump (`knowledge/devdocs/std_go_simpleredis.md`, `std_go_test-suites.md`) after apply.
- No runtime client API change. No new compose routers (PathPrefix already matches `/redis/<case>`). No Redis 8 / native Dragonfly MSETEX.
