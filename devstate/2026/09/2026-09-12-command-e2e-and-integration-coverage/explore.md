# Explore
IssueKey: 2026-09-12-command-e2e-and-integration-coverage

## Concepts

**Public command** — an exported SimpleRedis method that speaks Redis: Get, MGet, Set, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, MSetEXAt (`simpleredis/commands.go`, `commands_eval.go`, `commands_msetex.go`). Pool/config getters and `Close` are not commands.

**Go E2E** — `*_e2e_test.go` that talks to live Redis 7 and/or Dragonfly via `SIMPLEREDIS_LIVE_*` (`knowledge/devdocs/std_go_test-suites.md`). Compiled verb files plus Yaegi live (`yaegi_e2e_test.go`) share that skip/run rule. Pester is a different suite.

**Pester / Traefik probe** — `Test-Integration.ps1` + `e2e/simpleredisprobe/plugin.go`. Traefik v3 loads the plugin under Yaegi. Compose already uses `PathPrefix(`/redis`)` and `PathPrefix(`/dragonfly`)`, so `/redis/get` hits the same router with no new labels.

**Path case** — last path segment after the engine mount (`/redis/get` → `get`, `/dragonfly/msetexat` → `msetexat`). Exact `/redis` and `/dragonfly` stay a cheap Set+Get for compose health (`Test-Integration.ps1`). Handshake routers (`/redis-wrong-password`, `/dragonfly-database-99`, …) are different PathPrefix rules; they keep a single Set+Get that 502s. Unknown remainder on the success routers is 404 so a typo `It` fails.

**Verb header dump (DestBranch)** — `ServeHTTP` always runs every verb and sets many `X-SimpleRedis-*` headers; `Assert-SimpleRedisVerbHeaders` asserts them all on one GET. MSetEXAt is called with no header. That dump is what this change replaces.

```
compiled Go E2E                         Traefik Pester (this change)
(every public verb already called)     GET /redis/<case> and /dragonfly/<case>
        │                                      │
        │  Eval KEYS case + future MSetEXAt   │  one It per case, one result
Yaegi live LiveVerbs                     recover/hold/drop stay their own paths
Get Set Del Incr Eval MSetEX             handshake routers unchanged
missing: MGet IncrBy Expire ExpireAt MSetEXAt
```

## Decisions

- **“All commands” means the public verb set, not pool getters.** Get through MSetEXAt. Measured: compiled `TestLive_Commands` / `TestLive_Eval` / `TestLive_MSetEX` already call every one of those against both engines when both LIVE addrs are set (`simpleredis/commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go`). The ticket is not “add missing compiled calls for Get/Set/Del/…”.
- **Compiled Go E2E still needs two Traefik-shaped cases**, not new verbs: (1) `Eval` with KEYS/ARGV as a named case in `TestLive_Eval` (today only `Eval(..., nil, nil)` integer scripts; KEYS appear only inside `assertLiveTTLPositive`). (2) `MSetEXAt` with a future Unix time then Get + positive TTL (today only past EXAT → miss). Those are the argv shapes the probe already uses.
- **Yaegi live is in-scope.** `yaegi_e2e_test.go` is Go E2E (`std_go_test-suites.md`). `LiveVerbs` omits MGet, IncrBy, Expire, ExpireAt, MSetEXAt. Expand that helper (and the live-e2e / Yaegi spec lines that name the subset) so interpreted live hits the same public set. Fake-TCP Yaegi (`yaegi_test.go` Init/Get/Set/Del/Incr/Eval/MSetEX) stays the existing subset unless a verb cannot be proved without a live engine — do not grow the fake matrix for this ticket.
- **Traefik: HTTP map of SimpleRedis; Pester owns the sequences.** Human asked at `ServeHTTP`, then asked not to use result headers. `ServeHTTP` maps the last path segment to one public verb. Success is 200 + body; command errors are 502 + `err.Error()`. Pester composes Set then Get, Eval (script in the body), `drop=1`, recover after CLIENT KILL, and hold via TIME-wait Eval. Exact `/redis` and `/dragonfly` stay health Set+Get. No new compose routers. Handshake routers stay Set+Get 502. Domain files: `scripts/integration-tests.simpleredis.Tests.ps1` and `scripts/integration-tests.reclaim.Tests.ps1`; helpers in `scripts/integration-tests.utils/`.
- **Dragonfly has no native MSETEX.** `knowledge/research/ext_dragonfly_msetex/` — Lua fallback is the live path. No native-MSETEX e2e on Dragonfly. Redis 7 same. Do not pin a Redis 8.4 image in this change.
- **Token-bucket / windowcounter / reclaim Pester stay out.** No Traefik plugin uses those packages for this ticket (`requirement.md` Out of scope).
- **No new research folder.** Redis EVAL KEYS and Dragonfly MSETEX are already sourced. Devdocs `std_go_test-suites` and `std_go_simpleredis` already name the suites; update usage only when the Yaegi live verb list in those packets becomes wrong (implement / devdocsimpact).

## Open questions

- Q: Should Yaegi live `LiveVerbs` grow to the remaining public verbs (MGet, IncrBy, Expire, ExpireAt, MSetEXAt)?
  Rank: additive asked — adds cases on a helper this change already owns; Problem line “Live Redis/Dragonfly e2e must cover every public SimpleRedis command” and `yaegi_e2e_test.go` is that suite
  Decision: assumed — expand `LiveVerbs` and the Yaegi live spec line; leave fake-TCP Yaegi at the existing subset.
  By: explore

- Q: Should compiled `TestLive_Eval` add a KEYS/ARGV case matching the Traefik Kong snippet (or TTL KEYS), not only `return N` with nil keys?
  Rank: additive asked — Desired “close compiled e2e gaps”; Traefik already uses KEYS Eval; criterion is engine-success of the public Eval argv the probe uses
  Decision: assumed — add one compiled Eval KEYS case (Kong incrby+expireat or equivalent Lua 5.1-safe KEYS script). Keep the existing nil-keys integer / EVALSHA / FLUSH cases.
  By: explore

- Q: Should compiled `TestLive_MSetEX` add a future-EXAT landing (Get + positive TTL) in addition to past-miss?
  Rank: additive asked — Desired close compiled gaps; probe uses future MSetEXAt; live-e2e today only past miss
  Decision: assumed — add `msetexAtTTLLanded` next to `pastExatMiss`.
  By: explore

- Q: Native `MSETEX` on live Dragonfly v1.40.2?
  Rank: additive asked — MSetEX live path; criterion 3 names the Lua fallback already in the client
  Decision: resolved — unknown command then Lua; `knowledge/research/ext_dragonfly_msetex/`. Do not bake native Dragonfly MSETEX.
  By: explore

- Q: Expand `?recover=1` or drop-relay to the full verb set?
  Rank: additive incidental — listed under Out of scope
  Decision: resolved — do not. Pester composes recover as Set+Get after CLIENT KILL, drop as `drop=1` Incr+Eval, and hold as TIME-wait Eval. Do not add recover/drop/hold as extra probe verbs.
  By: explore

- Q: Replace the Traefik one-request-all-headers dump with a path-dispatched micro suite (middleware switches on path; one Pester `It` per case)?
  Rank: bounded asked — human asked at `e2e/simpleredisprobe/plugin.go:95`; 5 call-site groups enumerated and migratable here: `plugin.go` ServeHTTP, `scripts/integration-tests.simpleredis.Tests.ps1`, `Test-Integration.ps1` health URLs, `docker-compose.yml` PathPrefix (already matches `/redis/<verb>`; no new routers), `openspec/specs/std_go_simpleredis_resp-commands/spec.md` Traefik requirement and scenarios
  Decision: resolved — take HTTP verb paths. Last segment is the verb. Success is 200 + body; errors are 502 + `err.Error()`. Exact `/redis` `/dragonfly` remain health. Handshake routers stay Set+Get 502. Pester files split by domain.
  By: implement
