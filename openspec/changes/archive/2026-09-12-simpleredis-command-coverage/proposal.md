## Why

Compiled live e2e already calls every public SimpleRedis verb, but Yaegi live omits several, compiled Eval never names KEYS as its own case, and Traefik Pester used to dump every verb onto one GET so a 502 could not name the command. The probe is a thin HTTP map of SimpleRedis; Pester owns the sequences.

## What Changes

- Add compiled live cases: Eval with KEYS/ARGV (Kong incrby+expireat shape) and MSetEXAt with a future Unix time then Get + positive TTL.
- Expand Yaegi live `LiveVerbs` to MGet, IncrBy, Expire, ExpireAt, and MSetEXAt. Fake-TCP Yaegi stays the existing subset.
- Map each public verb to `/<engine>/<verb>` (query `key`/`arg`/`ex`/`at`/`delta`/`digest`, body for Set/Eval/MSetEX). Success is HTTP 200 with the reply body; a command error is 502 with `err.Error()`. Exact `/redis` and `/dragonfly` stay health Set+Get. Handshake routers stay Set+Get 502. Pester composes recover (Set+Get after CLIENT KILL), drop (`drop=1`), and hold (Eval of a TIME wait). No `X-SimpleRedis-*` result headers. Redis vs Dragonfly is `INTEGRATION_ENGINE` on one SimpleRedis file; CI splits `Integration Tests` (reclaim), `Integration Tests Redis`, and `Integration Tests Dragonfly`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_simpleredis_live-e2e`: Compiled Eval KEYS case, future MSetEXAt TTL landing, and Yaegi live covering the full public verb set.
- `std_go_simpleredis_resp-commands`: Traefik probe maps verbs to paths; Pester asserts status and body per request.
- `std_go_simpleredis_tcp-session`: Recover is Pester Set+Get after kill; drop-relay is `drop=1`; health `/redis` is Set+Get only.
- `std_go_ci_test-suites`: Pester CI is reclaim plus Redis and Dragonfly jobs that switch `-Engine`.

## Impact

- `simpleredis/commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`, `yaegi_test.go` `LiveVerbs`, `yaegi_e2e_test.go`.
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.simpleredis.Tests.ps1`, `scripts/integration-tests.utils/`, `Test-Integration.ps1` (`-Suite`, `-Engine`), `.github/workflows/ci.yml` Pester jobs.
- `openspec/specs/std_go_simpleredis_live-e2e/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`.
- Usage packets that name the Yaegi live subset or Traefik dump (`knowledge/devdocs/std_go_simpleredis.md`, `std_go_test-suites.md`) after apply.
- No runtime client API change. No new compose routers (PathPrefix already matches `/redis/<verb>`). No Redis 8 / native Dragonfly MSETEX.
