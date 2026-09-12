## Context

See proposal.md Why. Dest already has job `e2e` (`Go E2E`) that starts Redis and Dragonfly together. `lookupLiveEngineAddrs` fatals when exactly one LIVE address is set. Lint, Unit, Unit race, and Pester stay.

## Goals / Non-Goals

**Goals:**
- Two GitHub Checks: `Go E2E Redis` and `Go E2E Dragonfly`.
- Each live job starts only that engine (plus that engine’s AUTH sibling).
- Live helpers run the set engines; skip only `-short` or both unset.
- Keep dest port numbers (`6379`/`6381` Redis, `6380`/`6382` Dragonfly).

**Non-Goals:**
- A GitHub Actions matrix or reusable workflow.
- Extracting the three `lookupLiveEngineAddrs` copies into a shared package.
- Extra engine images. Runtime API changes. `-race` on live jobs. Changing Pester.

## Decisions

1. **Two explicit jobs, not a matrix.** Service containers and AUTH `docker run` differ per engine. Alternative: matrix still duplicates `services:` and env.

2. **Job ids `e2e-redis` / `e2e-dragonfly`, names `Go E2E Redis` / `Go E2E Dragonfly`.** Drop `e2e`. Alternative: keep id `e2e` with a matrix name — rejected (one workflow job id is not two Checks unless matrix; dest services cannot be matrix-conditional without the same copy).

3. **Return the set engines; drop `errLiveEngineOneAddr`.** Local `go test` with both pairs still runs both. Alternative: keep fail-on-one-addr and set dummy addrs in CI — that would still start one engine and lie about the other.

4. **AUTH splits with the same jobs.** Redis job starts `:6381` only; Dragonfly job starts `:6382` only. `TestLive_WrongPassword` uses the same helper.

5. **Keep dest ports.** Dragonfly stays `:6380` even when Redis is absent so env values and docs stay true.

6. **Copy YAML steps.** Do not add a reusable workflow this run (`skill:opd-commandments:Smallest durable delta`).

## Risks / Trade-offs

- [Two jobs double live-Go wall time] → accepted; that is the ticket (independent Checks). Timeout stays 5m per job.
- [Local one-addr no longer fails] → README and the catalog say one-engine runs are valid; both-unset still skips.
- [Three helper copies] → edit all three; do not extract this run.

## Migration Plan

Land on `master` via this PR. No runtime deploy. Rollback: restore job `e2e` and the both-or-neither helper.

## Open Questions

None. Job ids, AUTH split, ports, skip, and two explicit jobs are assumed on `devstate/explore.md`.
