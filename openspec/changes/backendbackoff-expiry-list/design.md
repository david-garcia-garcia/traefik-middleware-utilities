## Context

See proposal.md and `devstate/explore.md` (options 1-4). Option 1 is chosen: it is the shape #105 already measured in `tokenbucket/memory.go`. Options 3 (amortized partial sweep) and 4 (shards) do not reach O(1) at the cap. Option 2 (heap, ring) costs more allocations and method calls under Yaegi.

## Goals / Non-Goals

**Goals:**

- New-key admission O(1) amortized, compiled and Yaegi.
- Same observable behavior as the live `std_go_backendbackoff_allow` spec.
- `TestAllocAllowWarm` ceiling (warm CLOSED key) still holds.

**Non-Goals:**

- Preferring CLOSED over OPEN entries when evicting at the cap (spec says one arbitrary slot; out of scope).
- Importing `tokenbucket` (forbidden by spec) or sharing code with it.
- Changing Report/Allow signatures (sibling ticket).

## Decisions

1. **Copy the list mechanics, do not import.** `memEntry` gets `key string; prev, next *memEntry`. `Gate` gets `head, tail *memEntry`. Helpers mirror `tokenbucket/memory.go`: `storeLocked`, `removeLocked`, `unlinkLocked`, `pushBackLocked`, `moveToBackLocked` (relink in place, tail stays put), `makeRoomLocked(now)`. Keep `moveToBackLocked` inlined where `tokenbucket` inlined it for the Yaegi call cost.

2. **Report keeps refreshing the TTL and relinks.** Today `Report` writes `expireAt`; `TestReport_AfterTTLStillRecords` depends on it. Each write of `expireAt = now + TTL` is followed by `moveToBackLocked`. Alternative: stop writing `expireAt` in Report — rejected, behavior change outside this ticket.

3. **`loadEntry`.** Existing key expired → `removeLocked`. Missing key → `makeRoomLocked(now)` (pop expired heads; if `len(keys) >= maxMemorySources`, pop the head) → `storeLocked`. No `range g.keys` remains in non-test code except `Close` (which replaces the map).

4. **Clock going backwards.** Same caveat as `tokenbucket`: if `now` moves backwards, an entry refreshed later may sit behind one that expires sooner; expiry stays lazy and correct per key (an expired key found by lookup is still treated as unseen), only eviction order is approximate. Document in a code comment.

5. **Tests and benchmarks.** `backendbackoff/zzz_expiry_list_test.go`, helpers suffixed `ExpiryList`, with a test-only O(1) store (`storeEntryForTestExpiryList`) so prefilling 65,536 keys is O(n), not O(n²): (a) at the cap, expired keys drop first and `len(keys)` stays 65,536; (b) at the cap with none expired, one key is dropped and the new key is admitted; (c) Allow deny and Report move the entry to the tail (list order check); (d) list and map stay consistent after Close and reuse. Benchmarks `BenchmarkAllow/new-key-full-table` and `BenchmarkYaegiAllow/new-key-full-table` (or the existing bench files' naming), reported compiled and Yaegi in the delivery card next to the explore baseline (1.8 ms compiled at 65,536; 5.1 ms Yaegi at 16,384).

## Risks / Trade-offs

- [Three more pointer/string fields per entry] → ~32 B per key; bounded by the 65,536 cap (~2 MiB).
- [Warm path does one relink per Allow] → `moveToBackLocked` is a no-op when the entry is already the tail; `TestAllocAllowWarm` must keep its ceiling (relink allocates nothing).
- [Merge with `backendbackoff-probe-permit`] → both add fields to `memEntry` and touch `Allow`/`Report`; conflicts are mechanical.

## Migration Plan

Internal change, no API change. Rollback is revert.

## Open Questions

None.
