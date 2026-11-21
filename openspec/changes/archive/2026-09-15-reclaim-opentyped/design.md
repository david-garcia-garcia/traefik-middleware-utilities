## Context

See proposal.md — Why. Dest `Table.Open` owns the lookup loop and passes caller-supplied `Hooks` into `put`. `put` calls `create() (any, error)` and `publishPut` already takes hooks as a plain value. `go.mod` is `go 1.21`. Yaegi v0.16.1 cannot instantiate `Table[T]` from another package (`knowledge/research/ext_traefik_plugins_yaegi-generics`). Ready-made hunks live at `D:\tmp\reclaim-upstream-patch\` (vendor-prefixed `table.go.diff`; complete `opentyped.go`).

## Goals / Non-Goals

**Goals:**

- Thread hooks from `create` to `publishPut` so `EnforceCloseBeforeOpen` is the bool `create` returned.
- Keep `Open` byte-compatible in signature and error precedence.
- Give callers a typed helper that does not require `Table[T]`.

**Non-Goals:**

- Changing the slot state machine, `publishPut`, or grace/Close ordering.
- Making `Table` generic.
- Migrating `e2e/reclaimprobe` or existing `Open` tests.
- Tagging or bumping a module version.

## Decisions

- **OpenWithHooks owns the loop; Open wraps.** Alternative: keep the loop on `Open` and add a second copy. Rejected: one owner (`skill:opd-commandments:One job, one owner`). Alternative: change `Open`'s create to return hooks. Rejected: **BREAKING**.
- **`put` drops its hooks parameter.** Alternative: keep both and ignore one. Rejected: two sources of truth. `publishPut` stays unchanged.
- **Open checks nil create before wrapping.** The wrapper closure is never nil, so `OpenWithHooks` cannot see a nil `Open` create. Inside that branch, table and logger still win. Alternative: document a new error order. Rejected: dest compatibility.
- **OpenTyped is a package function, not a method.** Go methods cannot declare their own type parameters. Alternative: `Table[T]`. Rejected: Yaegi.
- **Instantiation stays a call expression.** Documented on `OpenTyped`. Package-level vars/aliases/fields of a foreign generic instantiation fail under Yaegi v0.16.1.
- **Starting point is the ready-made patch**, then match this package's doc-comment voice. Apply with the vendor prefix stripped (`git apply -p11` or hand hunks).
- **Tests stay in-package** (`testpackage` off). Yaegi case in `reclaim/yaegi_test.go`, skip under `-race` like the siblings.
- **FindSpecHost:** fold `std_go_reclaim_value-lifecycle` (high). Candidates: that leaf and `std_go_reclaim_context-lease`. Lease bind rules do not change; one delta.

## Risks / Trade-offs

- [Open wrapper misses a nil-create + nil-table case] → Regression tests for the three-way precedence, including combined nils.
- [Yaegi rejects a stored generic instantiation] → Helper stays a call; Table stays non-generic; harness case is a call expression.
- [Consumer still on dest Open after merge] → No tag this run. Note in the PR that `traefik-geoblock` needs a version bump plus `go mod vendor` and must re-apply `scripts/apply-oschwald-yaegi-patch.ps1`.

## Migration Plan

- Land on `master` via this PR. Existing `Open` callers compile unchanged.
- Do not tag. Downstream vendors when they choose.
- Rollback: revert the PR. `Open` is unchanged for dest callers.

## Open Questions

None.
