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
