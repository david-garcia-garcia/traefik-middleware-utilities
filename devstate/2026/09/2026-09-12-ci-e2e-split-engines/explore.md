# Explore

## Concepts

GitHub Check names on this repo are the `jobs.<id>.name` values. PR 41 [run 34703992540](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34703992540) lists five Checks: Lint, Unit, Unit race, **Go E2E**, Integration Tests. One live-Go item. That matches `.github/workflows/ci.yml` job `e2e` `name: Go E2E`, which starts Redis `:6379` and Dragonfly `:6380` plus AUTH `:6381`/`:6382` and sets every `*_LIVE_*` pair.

`lookupLiveEngineAddrs` in three packages (`simpleredis/simpleredis_e2e_test.go`, `windowcounter/limiter_e2e_test.go`, `tokenbucket/limiter_e2e_test.go`) returns `errLiveEngineOneAddr` when exactly one address is set. `liveEngineAddrs` then `t.Fatal`s. `go test -short -run TestLookupLiveEngineAddrs` passed in all three packages: `onlyRedis` / `onlyDragonfly` assert that error. A Redis-only CI job would fail those live tests today, before any engine case runs.

AUTH WRONGPASS uses the same helper on `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` (`simpleredis/pool_e2e_test.go` `TestLive_WrongPassword`). If those stay both-or-neither, splitting the unauthenticated pair is not enough.

Local `go test` with both pairs set must still run both engines in one invocation. That is dest’s local path; the ticket asked for two CI Checks, not to forbid a two-engine local run.

```
DestBranch CI
  Lint | Unit | Unit race | Go E2E (Redis+Dragonfly) | Integration Tests

This change
  Lint | Unit | Unit race | Go E2E Redis | Go E2E Dragonfly | Integration Tests
```

## Decisions

Two explicit GitHub Actions jobs, not a matrix: service containers and AUTH `docker run` differ per engine. Job ids `e2e-redis` / `e2e-dragonfly`, names `Go E2E Redis` / `Go E2E Dragonfly`. Drop job id `e2e`.

Each job starts only that engine (and that engine’s AUTH sibling). Ports stay dest numbers (`6379` / `6381` Redis, `6380` / `6382` Dragonfly) so env values keep their meaning.

`lookupLiveEngineAddrs` returns the engines whose addrs are set: both unset → skip; one set → that one; both set → both. Drop `errLiveEngineOneAddr`. Update the three `TestLookupLiveEngineAddrs` copies. Callers of `runForEachLiveEngine` stay as they are.

Do not extract the three helper copies into a shared package this run (dest already duplicated them). Edit all three (same skip table).

## Open questions

- Q: What GitHub Check `name:` / job ids should the two live items use?
  Rank: additive asked — Desired line 1 names two visible Checks, Redis vs Dragonfly, and does not quote the strings
  Decision: assumed — job ids `e2e-redis` and `e2e-dragonfly`; names `Go E2E Redis` and `Go E2E Dragonfly`. Remove job id `e2e`.
  By: explore

- Q: Do passworded AUTH containers (`:6381` / `:6382`) split with the same jobs?
  Rank: bounded asked — Desired line 1 splits the live-Go suite; AUTH is that suite’s WRONGPASS pair; 1 call site `TestLive_WrongPassword` plus the three `lookupLiveEngineAddrs` copies (roots: `*_e2e_test.go`)
  Decision: assumed — Redis job starts only `simpleredis-redis-auth` on `:6381` and sets `SIMPLEREDIS_LIVE_REDIS_AUTH`; Dragonfly job starts only `:6382` and sets `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH`. Leave the other AUTH env unset.
  By: explore

- Q: May a Redis job publish Redis on `:6379` only (no Dragonfly `:6380` on that runner)?
  Rank: additive asked — requirement Unknowns names this port choice
  Decision: assumed — yes. Redis job: Redis `:6379` + AUTH `:6381` only. Dragonfly job: Dragonfly `:6380` + AUTH `:6382` only. Keep dest port numbers.
  By: explore

- Q: When CI sets only one of Redis/Dragonfly, skip the other engine or fail?
  Rank: bounded asked — Desired line 2 says each live job may skip the other engine; DestBranch `lookupLiveEngineAddrs` fatals; three helper copies plus `TestLookupLiveEngineAddrs` in simpleredis, windowcounter, tokenbucket (roots: those three `*_e2e_test.go` files)
  Decision: assumed — return the set engines; skip only when `-short` or both unset; one addr set is a one-engine run, not a fail. Both addrs still run both (local). Rewrite the three `onlyRedis` / `onlyDragonfly` unit cases to expect one engine and nil error.
  By: explore

- Q: Two explicit jobs versus a GitHub Actions matrix?
  Rank: additive incidental — means to Desired line 1; matrix cannot express different `services:` / AUTH `docker run` without the same duplication
  Decision: assumed — two explicit jobs. Copy checkout/setup-go/`go test` steps. Do not add a reusable workflow this run.
  By: explore
