# Requirement
IssueKey: 2026-09-12-command-e2e-and-integration-coverage

## Problem
Live Redis/Dragonfly e2e must cover every public SimpleRedis command. Traefik integration (Pester + probe) must not miss a command or path that actually runs through Traefik.

## Current (code)
Public verbs on `SimpleRedis` are Get, MGet, Set (SET+EX), Del, Incr, IncrBy, Expire, ExpireAt (`simpleredis/commands.go`), Eval (`simpleredis/commands_eval.go`), MSetEX, MSetEXAt (`simpleredis/commands_msetex.go`). Package comment at `simpleredis/simpleredis.go:1` lists the same set.

Compiled live e2e (`runForEachLiveEngine` on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`):
- Get hit/miss, Set+EX then Get, Del, MGet hits/misses/empty, Incr then IncrBy, Expire then TTL, ExpireAt then TTL: `simpleredis/commands_e2e_test.go`
- Eval integer, second Eval (EVALSHA), Eval after SCRIPT FLUSH: `simpleredis/commands_eval_e2e_test.go` (`Eval(..., nil, nil)` only)
- MSetEX then Get + positive TTL; MSetEXAt past timestamp then Get miss: `simpleredis/commands_msetex_e2e_test.go`
- TTL helper Evals `TTL` with KEYS: `simpleredis/simpleredis_e2e_test.go` `assertLiveTTLPositive`
- Yaegi live `LiveVerbs`: Set, Get, Del, Incr, Eval (`return 1`, nil keys), MSetEX — not MGet, IncrBy, Expire, ExpireAt, MSetEXAt: `simpleredis/yaegi_test.go` `LiveVerbs`; `simpleredis/yaegi_e2e_test.go`

Traefik: `e2e/simpleredisprobe/plugin.go` `ServeHTTP` runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval twice (Kong KEYS script), Get miss, MSetEX + TTL Eval, then MSetEXAt. It sets a header per verb except MSetEXAt (call at `plugin.go` after `X-SimpleRedis-MSetEX-TTL`; no `X-SimpleRedis-MSetEXAt`). Pester `Assert-SimpleRedisVerbHeaders` in `scripts/integration-tests.Tests.ps1` asserts those headers on `/redis` and `/dragonfly`; it does not assert MSetEXAt. `?recover=1` is Set+Get only. Drop-relay path is Incr+Eval only. Compose routes: `docker-compose.yml` whoami `/redis` and `/dragonfly`. Runner: `Test-Integration.ps1`.

Spec: `openspec/specs/std_go_simpleredis_live-e2e/spec.md` lists the compiled verb set (Eval integer, MSetEX, past EXAT). `openspec/specs/std_go_simpleredis_resp-commands/spec.md` Traefik requirement says one response header per verb including MSetEXAt; the Pester scenario THEN lists Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, MSetEX-TTL and omits MSetEXAt. Yaegi live spec is New plus Get, Set, Del, Incr, Eval, MSetEX.

## Desired
Close compiled e2e gaps so each public command is proven against Redis and Dragonfly (both engines when both addrs are set). Review Traefik probe + Pester and close gaps that would miss a command or path the probe already uses (MSetEXAt header/assert is the one found). Do not add product features.

## Affected
- `simpleredis/commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`, `yaegi_e2e_test.go` / `yaegi_test.go` `LiveVerbs` (only if explore treats Yaegi live as in-scope for “all commands”)
- `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`
- Specs that name those proofs: `openspec/specs/std_go_simpleredis_live-e2e/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`

## Out of scope
- New Redis verbs or client APIs
- Token-bucket / windowcounter Traefik probes (no Traefik plugin uses those packages)
- Reclaim Pester (`e2e/reclaimprobe`)
- Fake-TCP / unit-only cases (malformed RESP, LOADING retry)
- Expanding `?recover=1` or drop-relay beyond the verbs those paths already exist to prove

## Unknowns
- Whether live Dragonfly v1.40.2 accepts native `MSETEX` (Redis 7 compose image does not; e2e comment in `commands_msetex_e2e_test.go` says Lua). Does not block listing the gap.
- Whether Yaegi live should grow to the full public verb set. Spec currently names a subset; ticket names the e2e suite that talks to Redis and Dragonfly.

## Tensions
- Ticket “all commands in e2e” vs spec Yaegi live subset (Get/Set/Del/Incr/Eval/MSetEX). Compiled files already call every public verb; Yaegi live does not.
- Spec Traefik “one header per verb” including MSetEXAt vs scenario THEN and Pester omitting that header. Probe calls MSetEXAt; failure would 502, success is unasserted.
