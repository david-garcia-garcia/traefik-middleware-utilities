## Why

Every unseen key makes `loadEntry` call `dropExpired`, which walks the whole `g.keys` map under `g.mu`; at the 65,536 cap `dropOne` walks it again. New-key cost grows with the map: measured 1 µs → 1.8 ms compiled (0 → 65,536 keys) and 36 µs → 5.1 ms under Yaegi (0 → 16,384). Everything is serialized on one mutex, so with high-cardinality keys about 270 new keys/s saturate the gate under Yaegi and every `Allow`/`Report` of every backend queues behind the scan. `tokenbucket.Memory` had the same walk and #105 replaced it with an O(1) expiry list; `backendbackoff` still has it.

## What Changes

- `memEntry` joins an intrusive doubly linked expiry list (`key`, `prev`, `next`), with `head`/`tail` on `Gate`. Concrete pointers, no `container/list`, same shape as `tokenbucket/memory.go`.
- Every write of `expireAt` (each `Allow`, including denies, and each `Report` that lands) moves the entry to the tail. The TTL is fixed per gate and the clock does not move backwards, so the head is always the next to expire.
- A new key drops expired entries from the head only, then, if still at the cap, drops the head. No full-map walk on any path.
- `Close` resets `head`/`tail` with the map.
- Observable behavior stays as `std_go_backendbackoff_allow` defines it (lazy TTL expiry, refresh on every Allow, cap 65,536, expired keys first, then one arbitrary slot). The head is a valid "arbitrary" choice; the spec is not tightened.
- Benchmarks for new-key admission at a full table, compiled and Yaegi; a cap/eviction test.
- Closes `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None. No live promise changes (`devstate/specs.md`: none — no live contract).

## Impact

- `backendbackoff/gate.go` (`memEntry` fields, `Gate` head/tail, `Close`), `backendbackoff/allow.go` (`loadEntry`, `Allow`/`Report` relink, remove `dropExpired`/`dropOne` walks).
- New `backendbackoff/zzz_expiry_list_test.go` and benchmark entries; `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md` deleted.
- Sibling `backendbackoff-probe-permit` adds a `gen` field to `memEntry` and changes `Allow`/`Report` signatures; keep this change's hunks to the list fields and the relink calls so the merge stays mechanical.
