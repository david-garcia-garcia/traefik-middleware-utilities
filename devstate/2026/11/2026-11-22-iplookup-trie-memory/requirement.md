# iplookup: one radix node per prefix bit makes large lists cost hundreds of MiB and load the GC

## Problem

`ipRadixTree.insert` walks a prefix bit by bit and allocates one `radixNode` (~48 B: bool, int, string, two pointers) per bit that is not shared. Random prefixes share only ~log2(N) top levels, so N prefixes create about `N × (bits − log2 N)` nodes: up to 32 per IPv4 /32 and up to 128 per IPv6 /128. Besides the memory, the tree is a graph of tens of millions of pointers that the Go GC must mark every cycle, which raises GC CPU and assist pauses for the whole Traefik process, not only this middleware. With GOGC=100 the heap target doubles the live heap. `Reset` followed by a rebuild keeps the old tree alive until the next GC (peak ≈ 2×).

`Contains` does not allocate; request traffic does not grow the tree. The trigger is the size of the stored list (blocklists / reputation feeds).

## Current (code, master 1f5ec454)

- `iplookup/tree.go:6-12` `radixNode`; `insert` / lookup / remove walk one bit per level.
- `iplookup/helper.go` `Helper` holds `ipv4Tree` / `ipv6Tree` under one `sync.RWMutex`; `Reset` at `:114`.

## Evidence (compiled)

| Prefixes | Heap | Bytes/prefix | Load |
|---|---:|---:|---:|
| 100k IPv4 /32 | 71 MiB | 747 | 92 ms |
| 1M IPv4 /32 | 560 MiB | 588 | 1.1 s |
| 100k IPv6 /128 | 472 MiB | 4,952 | 0.3 s |

Repro test: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\iplookup\zzz_scale_hunt_test.go` — `TestScale_TreeMemoryPerPrefix`.

## Desired

Memory per stored prefix and the number of heap pointers the GC must scan drop by at least an order of magnitude for large random host lists (IPv4 /32 and IPv6 /128), while keeping every current contract: family isolation, IPv4-mapped handling, longest-prefix match with metadata, duplicate replace without Count bump, RemoveCIDR, Reset, Count, concurrency safety (specs `std_go_iplookup_*`). Lookup latency must not regress materially; measure compiled and Yaegi (load time and Contains).

## Process note (binding for this run)

- Phases prepare and explore run first. Explore lists the options and open questions; it does NOT pick the final design.
- The propose phase (OpenSpec change) is written by the tech lead, not by the agent that ran explore. Do not start propose on your own.
- Report and evidence for all seven tickets of this batch: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\scale-bug-report.md` (read only the section for this ticket).

## Constraints

- The library runs compiled AND interpreted under Yaegi v0.16.1 (Traefik plugin). Yaegi-safe only: no `select` mixing per-call channels in package source, no `unsafe`, no `container/list` (any-typed Value), no package-level var / type alias / struct field naming a generic instantiation from another package. Existing benchmarks are reported compiled and Yaegi; any performance claim must show both.
- Test files are prefixed `zzz_` (#116). New test file(s) for this ticket are self-contained; helpers they need are suffixed with this ticket's slug so the seven parallel PRs never declare the same identifier. Do not create shared helper files. Tests assert what a correct implementation must do (no tautologies) and keep the control cases.
- Local gate before pushing: `go vet ./...` ; `go test -short -count=1 ./...` ; plus the touched package without `-short`. Local engines in Docker if a live test is needed: TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380 SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381 SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382 (password secret).
- CI on GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper, Unit, Unit race, e2e-redis, e2e-dragonfly) must be green.
- Six sibling tickets of this batch run in parallel on other branches. Touch only what this ticket needs. Three of them (`backendbackoff-*`) edit the same package: keep hunks minimal so merges stay mechanical.

## Current (code)

Grounded on this worktree at `origin/master` `35aaa343c5185f8d5315e8715c3ceb05caed247a` (HEAD of dest). Affected unit: package `iplookup`.

- `iplookup/tree.go:5-12` — `radixNode` is `isEndpoint bool`, `prefixLen int`, `metadata string`, `left` / `right *radixNode`.
- `iplookup/tree.go:24-53` — `insert` walks one bit per level via `bitAt` and allocates a child `radixNode` when that bit is missing; a second store of the same endpoint replaces `metadata` and returns `added=false`.
- `iplookup/tree.go:55-81` — `contains` walks existing children only and records the last `isEndpoint` (longest prefix). It does not create nodes.
- `iplookup/tree.go:83-126` — `remove` clears the endpoint and prunes empty leaves back toward the root.
- `iplookup/tree.go:128-158` — `familyWalk` / `prefixWalk` / `bitAt`: IPv4 and IPv4-mapped use `To4()` and 32 bits (mapped CIDR remaps `ones-96` when mask bits are 128); native IPv6 uses 128 bits.
- `iplookup/helper.go:15-20` — `Helper` holds `mu sync.RWMutex`, `ipv4Tree`, `ipv6Tree`, `count`.
- `iplookup/helper.go:38-51` — `AddCIDR` parses, locks, inserts on `treeFor(block.IP)`, bumps `count` only when `insert` reports added.
- `iplookup/helper.go:102-111` — `Contains` RLock and delegates to the family tree; nil IP is an error.
- `iplookup/helper.go:113-120` — `Reset` Lock, replaces both trees with `newIPRadixTree()`, sets `count=0`. The previous trees stay reachable only from locals that still hold them until GC.
- `iplookup/zzz_tree_test.go:29-44` — `TestContainsIPv4FourByteNoAlloc` requires 0 allocs for a 4-byte IPv4 query.
- `iplookup/zzz_helper_test.go:303-329` — `TestConcurrentResetAndContains` / `TestConcurrentContains`.
- `openspec/specs/std_go_iplookup_family-match/spec.md` — family isolation and IPv4-mapped remap.
- `openspec/specs/std_go_iplookup_cidr-store/spec.md` — AddCIDR / AddEntry / RemoveCIDR / Reset / Count / concurrent mutations with lookup.
- `openspec/specs/std_go_iplookup_match-label/spec.md` — longest-prefix metadata.
- `knowledge/devdocs/std_go_iplookup.md` — usage packet for the same contracts.
- Repro (not in this worktree): `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\iplookup\zzz_scale_hunt_test.go` — `TestScale_TreeMemoryPerPrefix`. Cited, not copied.

## Out of scope

- Atomic `ReplaceAll` / build-off-lock-and-swap (named in the hunt report as a related gap; Desired keeps `Reset` as the clear).
- Splitting the shared `RWMutex` or otherwise changing lock granularity.
- Copying the scale-hunt repro into this worktree during prepare.
- Picking the store representation in this phase. Explore lists options and open questions; propose (OpenSpec) is written by the tech lead.
- Sibling tickets of this batch (`backendbackoff-*`, `windowcounter-flush-scan`, and the other scale-hunt slugs).
- Product code, OpenSpec change folders, or explore notes in prepare.

## Unknowns

- Unmeasured runtime of `Contains` at 100k IPv6 /128 (pointer-chase / cache misses). The hunt did not measure lookup latency.
- Yaegi load time and `Contains` after any new representation. Hunt evidence is compiled only.
- Blast radius of replacing `ipRadixTree` internals versus keeping the `Helper` API and the three `std_go_iplookup_*` specs.
- Whether path compression (Patricia), a packed sorted-range store, or another Yaegi-safe form actually drops memory and GC pointers by ≥10× on large random host lists.
- Exact `radixNode` size on this Go/GC (`~48 B` is the hunt claim, not re-sized here).
- Peak ≈ 2× after `Reset` + rebuild: reachable from `Reset` replacing the tree pointers, not re-measured as heap here.
- Traefik-process GC CPU and assist pauses under `GOGC=100` with a live plugin: vendor/runtime, not measured in this worktree.
- The fix.

## Tensions

- Spec Current cites `master 1f5ec454`. This worktree dest is `origin/master` `35aaa343` (`#117` after `#116`). `radixNode` is still `iplookup/tree.go:6-12`. `Reset` is `iplookup/helper.go:113-120` (function starts at 113; spec said `:114`).
- Hunt report §2 "Dirección de arreglo" names Patricia, packed ranges, **and** `ReplaceAll`. Desired does not add `ReplaceAll`. That extra ask stays out of scope.
- Spec: "`Contains` does not allocate". `TestContainsIPv4FourByteNoAlloc` holds for 4-byte IPv4. `familyWalk` still calls `ip.To4()`, which can allocate a 4-byte slice for a 16-byte IPv4-mapped input. `contains` does not grow the tree.
- Hunt "No verificados" also names non-atomic `Reset`+`AddCIDR` (empty/partial visible to concurrent `Contains`) and RWMutex contention. Those are not this ticket's Desired; concurrency safety in the specs is "no panic / serialized mutations", not an atomic reload.
