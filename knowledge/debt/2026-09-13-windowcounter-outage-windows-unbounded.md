# Cap `windows` during buffered Redis outage

IssueKey: 2026-09-13-windowcounter-bug-buffered-outage
Size: large
Action: note

## Why this follow-up
During a Redis outage, buffered Take seeds `Limiter.windows` for every new opaque key and `localDelta` blocks expireAt eviction, so the map (and per-tick flush EVAL) can grow with unique keys until Redis recovers.

## Why it was not taken
This ticket is the per-node nil-error cap. Adding eviction or a max size is a new limiter contract, not Desired. Unattended take is only small rows on files this run created.

## Risks
A long outage with many distinct keys can hold memory and keep the flush ticker evaluating every pending key under `l.mu`.

## Context
Current: `windowcounter/limiter.go` `seedEmptyWindowLocked` / `flushPendingLocked`.
Proposed: reuse expireAt delete while a flush is failing, or a max entry count.
