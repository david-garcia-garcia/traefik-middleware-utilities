## Why

`ipRadixTree.insert` allocates one 48-byte `radixNode` per prefix bit that is not shared. Measured on dest: 100k IPv4 /32 take 71 MiB (748 B/prefix), ~1M IPv4 /32 take 560 MiB (588 B/prefix), 100k IPv6 /128 take 472 MiB (4,952 B/prefix). Besides memory, those are 10-12 million pointers per list that the Go GC marks on every cycle, which costs GC CPU for the whole Traefik process, and `Reset` plus a rebuild briefly doubles the peak. Large blocklists and reputation feeds are mostly single host addresses.

## What Changes

- Each family store keeps **host entries** (IPv4 /32, IPv6 /128, IPv4-mapped /128 remapped to IPv4 /32 as today) in a pointer-free Go map: `map[uint32]uint32` for IPv4 and `map[[16]byte]uint32` for IPv6, address → label id.
- **Labels** (metadata strings) are interned once per `Helper` in a table with reference counts and a free list, so host map values are small integers and the maps carry no pointers for the GC to scan. Label `""` is a permanent id.
- **Non-host prefixes** (shorter than /32 or /128) stay on the existing radix trie, unchanged.
- `Contains` checks the host map first (a host is the longest possible prefix of its family) and falls back to the trie. Longest-prefix match, metadata, family isolation, IPv4-mapped handling, duplicate replace without a `Count` bump, `RemoveCIDR`, `Reset`, `Count`, and the `RWMutex` stay as the live specs define them. Public API unchanged.
- Target: at least 10× less heap per prefix and per-entry GC pointers for large random host lists, compiled and Yaegi, without a material `Contains` regression.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None. No live promise changes (`devstate/specs.md`: none — no live contract). The three `std_go_iplookup_*` specs keep their wording.

## Impact

- `iplookup/helper.go` (fields, `New`, `Reset`, `AddCIDR`, `RemoveCIDR`, `Contains`), new `iplookup/store.go` (family store and label table). `iplookup/tree.go` keeps the trie for non-host prefixes.
- New tests (equivalence against a reference matcher, memory bounds, Yaegi coverage of IPv6 host keys) and benchmarks.
- Follow-up debt (note, not taken): path-compressing the trie for large lists of non-host prefixes.
