# Stop scanning every bucket when `tokenbucket.Memory` admits a new source

IssueKey: 2026-11-21-ratelimit-benchmarks
Size: large
Action: note

## Why this follow-up
`Memory.Allow` calls `dropExpired` (a full map walk) for every new source, and `dropOne` walks it again at the 65536 cap, all under the single mutex. Probe on 2026-11-21: about 0.8 ms per new source with 60k live sources, and 1.6 ms when the table is full. Every other `Allow` waits behind that lock.

## Why it was not taken
The ticket only adds benchmarks and README text. `requirement.md` puts optimising the limiters out of scope. Changing eviction also changes when a source loses its bucket, so it needs its own spec delta on `std_go_tokenbucket_allow`.

## Risks
High-cardinality traffic (many distinct client IPs) turns each first request from a new source into a stall for every request on that limiter.

## Context
Current: `tokenbucket/memory.go` `Allow` → `dropExpired`, `dropOne`.
Proposed: amortised expiry (sweep on a budget or a time interval, or keep an expiry-ordered structure) while keeping Traefik's 65536 cap.
