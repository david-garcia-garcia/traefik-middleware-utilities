# Design

## Context

Dest `*Memory` owns one `buckets` map (`tokenbucket/memory.go`); `NewMemory` allocates a new map. Dest `Redis.Allow` evals the caller `key` on the server (`tokenbucket/redis.go`); two `*Redis` on that key already share burst (`TestRedis_TwoInstancesShareBurst`, live `twoInstancesShare`). GoDoc and `std_go_tokenbucket_allow` name caller-owned keys, not Memory-per-instance vs Redis-per-key. `Allow` on dest is `(Status, error)` (`tokenbucket/status.go`). See proposal.md for why. Specs: `std_go_tokenbucket_allow` ADDED only. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Name the existing scope in GoDoc (`Memory`/`NewMemory`, `Redis`/`NewRedis`) and in the live catalog (one ADDED pair on `std_go_tokenbucket_allow`).
- Prove it with self-contained tests that pass: shared `*Memory` = `burst`; separate `*Memory` = `2*burst`; live Redis share = `burst`.
- Point callers at sharing one `*Memory` (example: `reclaim` Table keyed by middleware name) without editing `reclaim/`.

**Non-Goals:**
- Changing `Allow`, constructors, Lua, or the process map.
- A Traefik-style `scope: router | middleware` option, a process-wide Memory table, or prefixing `rate:` inside `tokenbucket`.
- Yaegi proofs for this issue, shared helpers without the `13706` suffix, or rewriting README (it already says one map per limiter instance).
- Wiring the consumer middleware (debt note already on disk).

## Decisions

1. **Fold ADDED into `std_go_tokenbucket_allow`; do not create a family.** Scope is an Allow contract (which store the opaque `key` hits), not a new leaf. Alternative: a `std_go_tokenbucket_scope` family — rejected; explore bound the catalog to this leaf. Alternative: GoDoc-only, no delta — rejected; Desired makes scope a documented contract and the catalog records the same promise. Alternative: MODIFY "Allow maps admit, wait, and deny" — rejected; that block is unchanged; this is an added concern.

2. **GoDoc transcribes the brief; `#13706` lives on the separate-instance subtest.** `Memory`/`NewMemory`: each value is its own key space; share one `*Memory` to share a budget across routers or reloads (example: `reclaim` keyed by middleware name). `Redis`/`NewRedis`: state lives on the server; any limiter on the same key shares it; the key prefix defines the scope. Alternative: cite the Traefik issue in GoDoc — rejected; the binding design puts that citation on test (b).

3. **Two test files, issue-suffixed helpers, dest `Status`.** `tokenbucket/traefik_issue13706_test.go`: parent test with subtests `sharedMemory` (`burst` total) and `separateMemory` (`2*burst`, comment cites `traefik/traefik#13706`); helper `countAdmits13706` reads `Status.Allowed`. `tokenbucket/traefik_issue13706_e2e_test.go`: live (c) via existing `runForEachLiveEngine` / `waitLiveClient`; helper `liveKey13706` = run nonce + `t.Name()` + suffix so it cannot collide with `TestLive_Limiter` keys (`t.Name()` only) or parallel PRs. Leave `TestLive_Limiter` / `twoInstancesShare` and `TestRedis_TwoInstancesShareBurst` as controls. Alternative: one failing repro that expects shared burst from two `NewMemory` — rejected; binding design asserts the documented contract. Alternative: extract shared `liveKey`/`countAdmits` — rejected; Tests forbid shared helper files. Alternative: Yaegi file — rejected; explore assumed compiled + live only.

4. **Usage gotcha, not Language and not README.** Implement adds the Memory-instance isolation line next to the existing Redis-share gotcha on `knowledge/devdocs/std_go_tokenbucket.md`. Language has no new term. Alternative: a new packet — rejected; the unit is already `std_go_tokenbucket`.

5. **Identity and middleware name stay with the caller.** Reuse the opaque `key` the caller already passes. Do not build Host, sourceCriterion, or a middleware-name prefix here. The `reclaim` key is also the consumer's. Alternative: a key builder in this package — rejected; live spec already says callers own identity.

## Risks / Trade-offs

- [Callers that expected two `NewMemory` to share one budget keep N budgets] → Mitigation: that is the documented contract; the consumer debt note names the middleware-level share. No behavior change, so no migration.
- [Live (c) duplicates `twoInstancesShare`] → Mitigation: keep the control; the new file is the issue-scoped, self-contained proof with `liveKey13706`.
- [Parallel issue PRs collide on helper names] → Mitigation: suffix `13706` on every helper this change declares.
- [GoDoc example names `reclaim` without importing it] → Mitigation: name it as an example of sharing a `*Memory`; do not import `reclaim` from `tokenbucket`.

## Migration Plan

Documentation and tests only. Rollback is revert. No Redis data change. Callers compile unchanged.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
