# Explore

## Concepts

Package `iplookup` is an in-process CIDR store. Callers pass `net.IP` they already selected. Helper does not reconstruct client address (spec `std_go_iplookup_family-match` “Caller owns the looked-up address”; usage Language **Helper**).

Units this change would touch:

| Name | Path | Job |
|---|---|---|
| Helper | `iplookup/helper.go` | Public CIDR store: `New`, `AddCIDR`, `AddEntry`, `HostCIDR`, `RemoveCIDR`, `Contains`, `Reset`, `Count`. Holds `mu sync.RWMutex`, `ipv4Tree`, `ipv6Tree`, `count`. |
| ipRadixTree | `iplookup/tree.go` | Unexported per-family bit trie. Candidate surface. |
| radixNode | `iplookup/tree.go:6-12` | One bit of a CIDR walk: `isEndpoint`, `prefixLen`, `metadata string`, `left`/`right *radixNode`. |
| familyWalk / prefixWalk / bitAt | `iplookup/tree.go:128-158` | Map address/CIDR onto the bit string; IPv4-mapped remap `ones-96` when mask bits are 128. |
| Live specs | `openspec/specs/std_go_iplookup_family-match/spec.md`, `std_go_iplookup_cidr-store/spec.md`, `std_go_iplookup_match-label/spec.md` | Family isolation, store/remove/reset/count/concurrency, longest-prefix label. |
| Usage packet | `knowledge/devdocs/std_go_iplookup.md` | How a middleware author calls Helper. |

```
plugin / test
    |
    v
 Helper  (public API — keep)
    |  RWMutex, count
    +-- ipv4Tree *ipRadixTree ----+
    +-- ipv6Tree *ipRadixTree ----+--> tree.go internals (candidate)
                                      radixNode per prefix bit
                                      familyWalk / prefixWalk / bitAt
```

Call sites (roots searched: worktree `*.go` and `*.md` for `ipRadixTree`, `newIPRadixTree`, `radixNode`, `iplookup.New`, import `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`; `iplookup/*.go` for `h := New()`):

- `ipRadixTree` / `radixNode` / `newIPRadixTree`: **10** production uses in `iplookup/helper.go` (2 fields, 4× `newIPRadixTree` in `New`/`Reset`, `treeFor`, `insert`, `contains`, `remove`). Types and methods live in `iplookup/tree.go`. **0** test files name those identifiers. **0** other packages.
- Helper public API in-tree production importers: **0** (only package `iplookup` itself). In-package tests: **20** `New()` (`zzz_helper_test.go` 16, `zzz_tree_test.go` 1, `zzz_entry_test.go` 3). Yaegi probe source in `zzz_helper_yaegi_test.go`: **3** `iplookup.New()`. Out-of-tree Traefik plugins would import Helper, not `ipRadixTree`.

Reproduce: **path run**, pass. Copied hunt test into `iplookup/zzz_scale_hunt_iplookup_trie_memory_repro_test.go`, ran `go test -count=1 -timeout 15m -v -run TestScale_TreeMemoryPerPrefix ./iplookup/` from the worktree, then deleted the file. `go version go1.25.6 windows/amd64`.

| Prefixes | Heap | Bytes/prefix | Load |
|---|---:|---:|---:|
| v6=false n=100000 | 71.3 MiB | 748 | 89 ms |
| v6=false n=999836 | 560.3 MiB | 588 | 1.072 s |
| v6=true n=100000 | 472.2 MiB | 4952 | 291 ms |

`n` is `h.Count()` after inserts (IPv4 /32 collisions under seed 1 drop 1M → 999836). Hunt table on `master 1f5ec454` was 71 MiB / 747 B / 92 ms; 560 MiB / 588 B / 1.1 s; 472 MiB / 4,952 / 0.3 s.

`unsafe.Sizeof(radixNode{})` = **48** (throwaway, deleted). `unsafe.Sizeof(ipRadixTree{})` = 8.

Contains at 100k IPv6 /128, compiled, 20k reps after load (throwaway, deleted): hit avg **106 ns** (last inserted address, warm path); miss avg **51 ns** (`2001:db8::1`). Yaegi Contains still unmeasured.

Outside facts used: hunt report §2 `iplookup` (read-only, in-tree sibling path); research `ext_traefik_plugins_yaegi-generics`, `ext_traefik_plugins_useunsafe`, `ext_traefik_plugins_yaegi-unsafe`, `ext_traefik_plugins_yaegi-select` (Yaegi v0.16.1, no `unsafe` under this product’s loader). No new `knowledge/research/` write.

## Decisions

- Chosen: keep Helper public API and the three live `std_go_iplookup_*` contracts; candidate surface is unexported `ipRadixTree` internals (`iplookup/tree.go` plus Helper’s two tree fields).
- Chosen: keep family isolation on Helper (`To4()` → IPv4 tree, including IPv4-mapped) and IPv4-mapped CIDR remap in `prefixWalk`; the store behind each family is the open pick.
- Chosen: explore lists representation OPTIONS only; propose (tech lead) picks the store form. Not Chosen here.
- Chosen: identity stays with the caller; Helper is not the owner of client address (do not reconstruct HTTP / X-Forwarded-For).
- Rejected: expand this ticket to atomic `ReplaceAll` / build-off-lock-and-swap — Out of scope; Desired keeps `Reset` as the clear.
- Rejected: split the shared `RWMutex` or change lock granularity — Out of scope.
- Rejected: pick Patricia or packed ranges (or any other form) in explore — Out of scope “Picking the store representation in this phase”.
- Rejected: commit the scale-hunt repro or a new test suite in this phase — reproduce then delete.
- Live contract: `std_go_iplookup_family-match`, `std_go_iplookup_cidr-store`, `std_go_iplookup_match-label`.

## Open questions

- Q: Which Yaegi-safe representation can drop memory and GC pointers ≥10× for large random host lists (IPv4 /32, IPv6 /128) while keeping the live contracts?
  Rank: structural asked — rewrite of the existing radix store component (`ipRadixTree` / one `radixNode` per bit); Desired names drop memory and GC pointers by at least an order of magnitude while keeping every current contract
  Decision: assumed — defer pick to propose (tech lead). OPTIONS, not Chosen:
  1. Patricia / path compression — `insert` (`iplookup/tree.go:24-53`) allocates one node per prefix bit; hunt + this reproduce: ~12M nodes for ~1M IPv4 (588 B/prefix), ~10M nodes for 100k IPv6 (4952 B/prefix). Patricia is ~2N nodes. IPv6 node-count drop is tens of ×; IPv4 is ~6× (12N→2N) and nodes stay a pointer graph, so ≥10× memory on IPv4 host lists is not guaranteed from node count alone. Yaegi-safe if concrete structs, no `unsafe`.
  2. Packed sorted ranges — hunt “Dirección de arreglo”: slice of `uint32` (IPv4) or `[2]uint64` (IPv6) plus binary search, ~8–32 B/entry, no per-node pointer graph for the GC. Versus 588–4952 B/prefix measured here → theoretically ≥10× memory. Longest-prefix with overlapping lengths, `RemoveCIDR`, and duplicate replace without Count bump need extra identity (disjoint segments with winning label, or a second index). Yaegi-safe: slices of concrete structs; no `unsafe`; no cross-package generic instantiation (`ext_traefik_plugins_yaegi-generics`).
  3. Other — hash of exact hosts plus a prefix list (Desired is host lists, but `std_go_iplookup_match-label` still requires mixed-length LPM); ART/stride trie (fewer hops, still pointers). Propose weighs them; none is Chosen here.
  By: explore

- Q: What is the Yaegi load time and Contains cost of a new representation?
  Rank: additive asked — measurement of a representation this change would introduce; Desired names measure compiled and Yaegi (load time and Contains)
  Decision: assumed — this reproduce and the hunt table are compiled only (`go test` on `go1.25.6`). Existing `zzz_helper_yaegi_test.go` proves contract roundtrips, not 100k-prefix load. Do not claim a Yaegi win from compiled 89 ms / 1.072 s / 291 ms load or 106 ns / 51 ns Contains. Measure both engines after propose picks a form.
  By: explore

- Q: What is Contains latency at 100k IPv6 /128?
  Rank: additive asked — no existing contract change; Desired names lookup latency must not regress materially; requirement Unknowns named this unmeasured
  Decision: resolved (compiled) — 20k reps after load on this machine: hit avg 106 ns (last inserted /128, warm path), miss avg 51 ns (`2001:db8::1`). Caveat: last path is hot from insert; not a Traefik mixed workload. Yaegi still unmeasured (see previous row).
  By: explore

- Q: What is the blast radius of replacing `ipRadixTree` internals versus keeping the Helper API?
  Rank: bounded asked — enumerated in-tree callers (10 `ipRadixTree` uses in `helper.go`, 0 other packages, 0 test names of the unexported type; 0 production importers of Helper); Desired keeps Helper and the three `std_go_iplookup_*` specs
  Decision: resolved — replace unexported internals only; keep Helper signatures and the three live specs. Out-of-tree plugins import Helper, not `ipRadixTree`. `familyWalk` / `prefixWalk` stay the family-map seam unless propose shows the new store absorbs that remap without changing Helper.
  By: explore
