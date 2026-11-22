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
