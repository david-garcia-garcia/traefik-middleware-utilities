## Context

DestBranch has no `iplookup/` package. Geoblock PR #86 (`7f8e32d:pkg/iplookup/iplookup.go`) already splits IPv4/IPv6 onto two `ipRadixTree` roots and routes with `To4()`. See proposal.md for why this module hosts that helper plus remove, label, and reset. Specs: `std_go_iplookup_family-match`, `std_go_iplookup_cidr-store`, `std_go_iplookup_match-label`. Explore: `devstate/2026/09/2026-09-15-iplookup-cidr/explore.md`. Yaegi: stdlib only, concrete types, no generics, no unsafe.

## Goals / Non-Goals

**Goals:**
- One `Helper` type, mutex, two family trees. Port insert/contains bit walking from geoblock; add endpoint metadata, exact-prefix remove with prune, and Reset.
- Prove with `-short` plus Yaegi interp. Family-collision cases from PR #86 stay as tests.

**Non-Goals:**
- `file_monitor`, geoblock `decide`, directory list loaders.
- Pester plugin, `*_LIVE_*`, e2e job edits.
- Reconstructing client IP from HTTP.
- A slice constructor (`NewIpLookupHelper([]string)`); callers loop `AddCIDR`.

## Decisions

1. **Type `Helper`, `New() *Helper`, methods `AddCIDR`, `RemoveCIDR`, `Contains`, `Reset`, `Count`.** House names match `reclaim.Table` / `backendbackoff.Gate`. Alternative: keep geoblock `IpLookupHelper` / `IsContained` — rejected; a second naming shape for the same job. Recorded as a deviation.

2. **Two unexported `ipRadixTree` values (`ipv4Tree`, `ipv6Tree`) plus `count` under one `sync.Mutex`.** `treeFor` uses `ip.To4() != nil`. Alternative: family bit on a shared root — rejected; geoblock already showed the shared first 32 levels leak across families, and `/0` of both families would collide on one endpoint.

3. **Endpoint holds `present bool` and `metadata string`.** Longest-prefix `contains` returns the deepest present endpoint. Alternative: metadata on every walked node — rejected; the match string is the winning CIDR's.

4. **Canonical prefix = `net.ParseCIDR` network IP + mask length.** `10.0.0.1/8` and `10.0.0.0/8` are the same store key. Count increments only when `present` flips false → true.

5. **RemoveCIDR walks to that length, clears `present`, then prunes nodes with no children and no endpoint.** Overlapping longer/shorter prefixes stay. Alternative: rebuild the tree — rejected; prune is local.

6. **Reset replaces both trees with empty roots and sets Count to 0.** Alternative: walk-delete every prefix — rejected; drop-and-replace is the full clear.

7. **Contains signature `(found bool, prefixLen int, metadata string, err error)`.** One lookup method. Alternative: `IsContained` plus a second metadata getter — rejected; two jobs for one match.

8. **Yaegi test copies non-test `iplookup` sources into a GOPATH interp (stdlib only), no unsafe.** Follow `backendbackoff/gate_yaegi_test.go`. Prove AddCIDR + Contains of a same-family hit and a cross-family miss.

9. **No `SetNowForTest`.** The helper has no clock.

## Risks / Trade-offs

- [A naive shared radix still matches across families] → Mitigation: two trees; tests `1.2.3.4/32` vs `102:304::1` and `808:808::/32` vs `8.8.8.8`.
- [Remove leaves a dangling endpoint that still matches] → Mitigation: clear `present` then prune; tests overlapping /8 and /16.
- [Concurrent Reset vs Contains races the tree] → Mitigation: one mutex on every exported method.
- [Yaegi interp misses a compile-only test] → Mitigation: family miss and label override in `helper_yaegi_test.go`.

## Migration Plan

New package. Callers opt in. Rollback is revert. Geoblock is not migrated in this change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
