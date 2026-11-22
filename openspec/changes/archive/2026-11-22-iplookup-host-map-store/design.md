## Context

See proposal.md and `devstate/explore.md`. Options: (1) Patricia/path compression — ~2N nodes; for IPv4 /32 lists about 6× fewer nodes and still a pointer graph, so ≥10× is not assured. (2) Sorted packed ranges — compact and pointer-free, but longest-prefix with overlapping lengths, `RemoveCIDR`, and duplicate replace need a second index, and every insert into a sorted slice is O(N). (3) Host map plus prefix structure — host lists (the large case) become pointer-free map entries; mixed-length LPM stays on the existing trie, which is small for real lists of ranges.

## Goals / Non-Goals

**Goals:**

- ≥10× less heap per prefix and per-entry GC pointers for large random IPv4 /32 and IPv6 /128 lists.
- Every live contract of `std_go_iplookup_family-match`, `std_go_iplookup_cidr-store`, `std_go_iplookup_match-label` unchanged.
- Yaegi-safe; measured compiled and Yaegi.

**Non-Goals:**

- Compressing the trie for large lists of non-host prefixes (follow-up debt).
- Atomic bulk replace, lock splitting, public API changes.

## Decisions

1. **Option 3: host maps + existing trie.** A family store holds `hosts` (IPv4: `map[uint32]uint32`; IPv6: `map[[16]byte]uint32`) and `tree *ipRadixTree`. `prefixWalk` keeps deciding family and the IPv4-mapped remap; a prefix whose length equals the family maximum (32/128) goes to `hosts`, anything shorter to `tree`. Alternative (1) — rejected, IPv4 host lists stay a pointer graph and miss ≥10×. Alternative (2) — rejected, O(N) inserts and extra indexes for remove/replace.

2. **Label interning per Helper.** `labelID map[string]uint32`, `labelText []string`, `labelRefs []uint32`, `labelFree []uint32`. Id 0 is `""` and is never freed. Insert of a new host: intern (ref++); replace: ref the new label then unref the old one; remove: unref; an id whose ref reaches 0 goes to the free list and leaves `labelID`. `Reset` replaces the table. Host map values are ids, so the maps hold no pointers. Trie nodes keep their `metadata string` as today.

3. **Lookup.** `Contains`: family by `To4()` as today; IPv4 key = big-endian `uint32` of the 4 bytes, IPv6 key = the 16 bytes copied into a `[16]byte` (no heap allocation). Host hit → `(true, 32|128, labelText[id])`. Miss → `tree.contains`. A host is the longest possible prefix, so checking it first preserves longest-prefix match.

4. **Count and duplicates.** `Count` = sum of host map sizes plus trie endpoints, maintained exactly as today (`added` only for a new canonical prefix). A /32 and a /31 covering it are different prefixes and both count, as today.

5. **Yaegi.** `[16]byte` map keys must load and work under Yaegi v0.16.1. The Yaegi test covers IPv6 and IPv4 host add, replace, contains, remove, and reset. If Yaegi rejects array keys, use `struct{ hi, lo uint64 }`; if that also fails, `map[uint64]map[uint64]uint32`. Record which one shipped and why in the delivery card.

6. **Tests and benchmarks.** `iplookup/zzz_host_map_store_test.go`, helpers suffixed `HostMapStore`: (a) equivalence — a seeded random mix of IPv4/IPv6 hosts, IPv4-mapped literals, and shorter prefixes with labels; 100k random lookups compared with a naive reference (linear scan, longest `net.IPNet.Contains` wins, label of that prefix), repeated after random removals, replacements, and a `Reset`; (b) memory — heap delta per prefix for 100k random IPv4 /32 ≤ 75 B and 100k random IPv6 /128 ≤ 100 B (dest: 748 / 4,952), measured with `runtime.GC` + `HeapAlloc` like the scale-hunt repro; (c) label refcount — replacing and removing every host returns the label table to only `""`. Benchmarks: load and `Contains` hit/miss for 100k IPv4 and IPv6 hosts, compiled and Yaegi, against dest numbers (Contains IPv6 compiled hit 106 ns, miss 51 ns).

## Risks / Trade-offs

- [A list of many non-host prefixes still uses the old trie] → Out of scope; note large in `knowledge/debt/` (path compression).
- [Labels unique per entry keep one string per entry] → Still one pointer per entry instead of 12-111; refcounts free them on remove.
- [Go maps do not shrink after mass removal] → Same as any map; `Reset` replaces them, which is the documented reload path.

## Migration Plan

Internal change, public API unchanged. Rollback is revert.

## Open Questions

None.
