# Geoblock PR #86 — IpLookupHelper family-isolated radix trees

Source: `https://github.com/david-garcia-garcia/traefik-geoblock` PR #86, HEAD `7f8e32df0be83b5b4f6879aa9dce775b4c5f7ba6` (temp clone `pr-86`, read-only).

## Package to port (core)

| Path | Role |
|---|---|
| `pkg/iplookup/iplookup.go` | `IpLookupHelper`, `ipRadixTree`, `AddCIDR`, `IsContained`, dual `ipv4Tree` / `ipv6Tree` |
| `pkg/iplookup/iplookup_test.go` | Unit tests including cross-family collision cases |

Target layout in this repo: top-level `iplookup/` (same pattern as `reclaim/`, `simpleredis/`).

## Family isolation (PR #86 fix)

On geoblock master before PR #86, one radix root let IPv4-mapped walks share the first 32 bit levels with IPv6 walks, so colliding prefixes could match the wrong address family.

PR #86 keeps `ipRadixTree` family-agnostic and gives `IpLookupHelper` two trees. `treeFor(ip)` picks the tree using `ip.To4() != nil` (IPv4 and IPv4-mapped IPv6 → IPv4 tree).

Public API on PR head (no metadata / remove / reset yet):

- `NewEmptyIpLookupHelper()`, `NewIpLookupHelper([]string)`
- `AddCIDR(string) error`, `Count() int`
- `IsContained(net.IP) (bool, int, error)` — longest-prefix match via `(found, prefixLen)`

## Related source not required by the local ticket text

| Path | Role |
|---|---|
| `pkg/iplookup/file_monitor.go` | One-shot directory `.txt` loader around `IpLookupHelper` — geoblock config wiring |

Port scope for this ticket is the reusable lookup helper, not geoblock plugin wiring.

## Specs on geoblock PR #86

Live spec id after archive: `core_geoblock_iplookup_family-match` (geoblock catalog). This utilities repo will need its own OpenSpec ids on propose.

## Module note

Source module: `github.com/david-garcia-garcia/traefik-geoblock`. Destination: `github.com/david-garcia-garcia/traefik-middleware-utilities`.
