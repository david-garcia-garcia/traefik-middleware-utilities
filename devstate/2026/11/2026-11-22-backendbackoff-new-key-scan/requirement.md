# backendbackoff: every new key scans the whole key map under the gate mutex

## Problem

When `Allow` sees an unknown key, `loadEntry` calls `dropExpired`, which walks the whole `g.keys` map under `g.mu`. At the 65,536 cap, `dropOne` walks it again and then deletes an arbitrary entry. Growing from 0 to 65,536 keys costs ~2.1 billion entry visits. Everything is serialized on one mutex, so the gate can admit at most `1 / cost` new keys per second, and the cost grows with the map. When saturated, every `Allow` and `Report` of every backend queues behind the scan. This is exactly the pattern #105 replaced in `tokenbucket.Memory` with an O(1) expiry list; `backendbackoff` still has it.

Only relevant when the key has high cardinality (client, IP, dynamic route). Keys per backend (tens/hundreds) are not affected.

## Current (code, master 1f5ec454)

- `backendbackoff/allow.go:96-125` `loadEntry` → `dropExpired` on every new key, `dropOne` at cap.
- `backendbackoff/allow.go:128-146` `dropExpired` / `dropOne` full-map loops.
- Reference fix shape: `tokenbucket/memory.go` (#105) expiry list with concrete pointers (`makeRoomLocked`, `moveToBackLocked`, …).

## Evidence (cost per new key)

| Resident keys | Compiled | Yaegi |
|---:|---:|---:|
| 0 | 1 µs | 36 µs |
| 4,096 | 32 µs | 1.1 ms |
| 16,384 | 145 µs | 4.7 ms |
| 65,000 | 1.3 ms | ~18 ms (extrapolated) |
| 65,536 (cap) | 2.1 ms | ~37 ms (extrapolated) |

Under Yaegi ~270 new keys/s saturates the mutex (map at 16k with TTL 60 s); compiled saturates near the cap at ~500/s.

Repro tests: `D:\repositories\traefik-middleware-utilities-2026-11-22-scale-hunt\repros\backendbackoff\zzz_scale_hunt_test.go` — `TestScale_GateNewKeyCostGrowsWithMapSize`; `...\zzz_scale_hunt_yaegi_test.go` — `TestScale_YaegiGateNewKeyCost`. Ignore the other tests in that file (other tickets).

## Desired

New-key admission is O(1) amortized in compiled and Yaegi, with the same observable behavior the spec `std_go_backendbackoff_allow` defines (lazy TTL expiry, refresh on every Allow including denies, cap 65,536, expired keys drop first). Benchmarks show compiled and Yaegi numbers.

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

Grounded on this worktree (`origin/master`), not on the SHA named in the dump heading.

- `backendbackoff/allow.go:95-115` `loadEntry`: a present key whose `expireAt` has elapsed is deleted; a missing or just-dropped key calls `dropExpired`, then `dropOne` when `len(g.keys) >= maxMemorySources`, then inserts a new CLOSED entry with credit `g.budget`.
- `backendbackoff/allow.go:128-135` `dropExpired`: full `range g.keys` under the caller's `g.mu` (held for the whole `Allow`).
- `backendbackoff/allow.go:137-147` `dropOne`: calls `dropExpired` again, then deletes the first key of map iteration order (`for key := range g.keys { delete; return }`).
- `backendbackoff/allow.go:7-39` `Allow`: `g.mu.Lock` for the entire call; every new key pays the scan while other `Allow`/`Report` wait on the same mutex.
- `backendbackoff/allow.go:42-93` `Report`: same `g.mu`; looks up `g.keys[key]` and returns nil if missing; does not call `dropExpired`/`dropOne`; refreshes `expireAt` on a present entry.
- `backendbackoff/gate.go:29` `maxMemorySources = 65536`.
- `backendbackoff/gate.go:53-62` `Gate` has one `mu` and `keys map[string]*memEntry`.
- `backendbackoff/gate.go:64-74` `memEntry` has `expireAt` and no `key`/`prev`/`next` list links.
- `tokenbucket/memory.go:9-35` reference shape: `memEntry` list pointers and `Memory.head`/`tail`.
- `tokenbucket/memory.go:105-114` `makeRoomLocked`: pop expired heads, then the head at cap (O(1) per new source).
- `tokenbucket/memory.go:144-165` `moveToBackLocked`: concrete pointers, not `container/list`.
- `openspec/specs/std_go_backendbackoff_allow/spec.md:46-47` live contract: lazy TTL on a later Allow, refresh on every Allow including denies, cap 65536, expired keys drop first, then one arbitrary slot; package MUST NOT import `tokenbucket`.
- `knowledge/devdocs/std_go_backendbackoff.md` usage packet: Gate is in-memory, no ticker, do not import `tokenbucket`; does not mention new-key scan cost.
- `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md` existing note: Gate still walks the map on every new key after #105 changed `tokenbucket.Memory`.
- `backendbackoff/zzz_bench_test.go` `BenchmarkAllowWarm` / `TestAllocAllowWarm`: warm existing CLOSED key only. New-key / full-table benches: not found.
- Cap/eviction unit test at 65536 in `backendbackoff/`: not found.
- `TestScale_GateNewKeyCostGrowsWithMapSize` / `TestScale_YaegiGateNewKeyCost` in this worktree: not found (live in the scale-hunt sibling, outside this dest tree).

## Out of scope

- Prefer CLOSED (or non-OPEN) entries when evicting at the cap. The dump Desired keeps `std_go_backendbackoff_allow` ("one arbitrary slot"). That preference is only in the scale-hunt evidence "Dirección de arreglo".
- The other six tickets of the scale-hunt batch (`windowcounter` flush scan, `iplookup` tree, stale Report probe, jitter point mass, `reclaim` OpenTyped leak, concurrent Sleep).
- Importing `tokenbucket` from `backendbackoff`.
- Shared helper files across the parallel PRs.
- Changing `Allow`/`Report` signatures, HTTP mapping, or cooldown math.
- Picking the final design in explore (process note: tech lead writes propose).

## Unknowns

- Unmeasured in this checkout: the dump cost table and saturation rates were taken in the scale-hunt sibling; this worktree does not contain those tests.
- Whether an expiry-ordered list copied from `tokenbucket/memory.go` is the only Yaegi-safe way to make new-key admission O(1) amortized. Explore lists options; it does not pick.
- Blast radius of adding list pointers to `memEntry` (tests that construct `memEntry` by hand: not found in `backendbackoff/*_test.go`).
- Whether "one arbitrary slot" stays unspecified after a list (tokenbucket's live spec leaves which live source is removed unspecified while the code pops LRU).
- Whether this run closes `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`.
- Exact compiled and Yaegi benchmark names needed to show the Desired numbers.

## Tensions

- scale-bug-report.md § 3 "Al expulsar en el tope, preferir entradas CLOSED antiguas" vs dump Desired "same observable behavior" / `std_go_backendbackoff_allow` "one arbitrary slot". The Desired wins; the preference is out of scope unless propose reshapes it.
- Dump heading cites master `1f5ec454`. This worktree is `origin/master` as grounded by prepare; line numbers above are this tree, not that SHA.
- Process note forbids explore from picking the design and forbids this run from starting propose. That is process, not a product gap.
- Existing debt note (`knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`) already names this same scan; this ticket is the take, not a second product ask.
