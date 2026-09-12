## Why

GitHub Checks show one `Go E2E` item that starts Redis and Dragonfly together. A Redis-only live failure is the same red check as a Dragonfly-only failure. Live helpers still `Fatal` when exactly one LIVE address is set, so the two Checks cannot run.

## What Changes

- Replace CI job `e2e` (`Go E2E`) with `e2e-redis` (`Go E2E Redis`) and `e2e-dragonfly` (`Go E2E Dragonfly`).
- Redis job starts Redis `:6379` plus AUTH `:6381` and sets Redis LIVE env only. Dragonfly job starts Dragonfly `:6380` plus AUTH `:6382` and sets Dragonfly LIVE env only.
- Live helpers return the engines whose addresses are set: skip when `-short` or both unset; one addr is a one-engine run; both addrs still run both (local).
- Keep Lint, Unit, Unit race, and Pester as they are. No `-race` on the live jobs.
- Update `std_go_test-suites.md` and README Tests so Go E2E is two Checks, not one both-or-neither job.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_ci_test-suites`: Two live-Go jobs instead of one `e2e`; skip/run when one engine addr is set.
- `std_go_simpleredis_live-e2e`: One-addr is a one-engine run; AUTH pair splits with the jobs.
- `std_go_simpleredis_resp-commands`: Live MSetEX runs on each engine job, not one `e2e` that sets both.
- `std_go_simpleredis_tcp-session`: Compiled pool-wait and CLIENT KILL run on each engine job; one-addr is not a fail.
- `std_go_windowcounter_sync-flush`: Live skip/run per set addr; CI is `e2e-redis` / `e2e-dragonfly`.
- `std_go_tokenbucket_lua-eval`: Live skip/run per set addr; CI is `e2e-redis` / `e2e-dragonfly`.

## Impact

- `.github/workflows/ci.yml` (split `e2e` into two jobs; services, AUTH `docker run`, env).
- `lookupLiveEngineAddrs` / `TestLookupLiveEngineAddrs` in `simpleredis/simpleredis_e2e_test.go`, `windowcounter/limiter_e2e_test.go`, `tokenbucket/limiter_e2e_test.go`.
- `knowledge/devdocs/std_go_test-suites.md`; README Tests.
- No runtime API change. No new engine images. Pester compose unchanged.
