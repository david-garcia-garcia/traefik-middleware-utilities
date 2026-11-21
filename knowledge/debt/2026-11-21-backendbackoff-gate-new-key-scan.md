# Give `backendbackoff.Gate` the O(1) expiry list that `tokenbucket.Memory` now uses

IssueKey: 2026-11-21-ratelimit-benchmarks
Size: large
Action: note

## Why this follow-up
`backendbackoff/allow.go` was copied from the `tokenbucket/memory.go` shape (`openspec/changes/archive/2026-09-12-backendbackoff-gate/tasks.md` 1.1): same 65536-key cap and a fixed `expireAt = now + TTL` on every use. This change replaces that shape in `tokenbucket.Memory` with an expiry-ordered list (O(1) per new source), but `Gate.loadEntry` still calls `dropExpired` / `dropOne`, which walk the whole key map under the mutex on every new key. The two siblings now diverge and the gate keeps the cost this ticket measured on the token bucket (about 1.5 ms per new source with a full table, `bench-before.txt` `BenchmarkMemoryAllow/new-key-full-table`).

## Why it was not taken
`requirement.md` names the rate limiters only; `backendbackoff` is a backend health gate outside this ticket, and the code review runs unattended, so it does not change a package the diff does not touch.

## Risks
A backend gate keyed by many distinct values pays a full map walk under its mutex per new key once the table is full, stalling every other key of that gate.

## Context
Current: `backendbackoff/allow.go` `Gate.loadEntry`, `dropExpired`, `dropOne`.
Proposed: the `tokenbucket/memory.go` list (`key` / `prev` / `next` on the entry, `head` / `tail` on the gate, `makeRoomLocked`, `storeLocked`, `removeLocked`, `moveToBackLocked`), plus a new-key-full-table benchmark compiled and under Yaegi.
