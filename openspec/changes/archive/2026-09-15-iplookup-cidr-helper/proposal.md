## Why

On DestBranch this module has no reusable CIDR store. Geoblock's helper after PR #86 isolates IPv4 and IPv6 so colliding prefixes cannot match across families, but that code lives in geoblock, and it still cannot remove a prefix, attach a string to a match, or clear the store.

## What Changes

- New package `iplookup/`: `Helper` with two family trees (`To4() != nil` → IPv4). Import path `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`.
- `New`, `AddCIDR(cidr, metadata)`, `RemoveCIDR`, `Contains`, `Reset`, `Count`. Same CIDR stored again replaces metadata and does not bump `Count`.
- `Contains` takes caller-owned `net.IP`. The library MUST NOT read HTTP, X-Forwarded-For, or Host.
- `go test -short ./iplookup/` plus interpreted Yaegi. No Pester plugin, no `file_monitor`, no geoblock `decide`.
- Usage packet `knowledge/devdocs/std_go_iplookup.md`. README Layout row and a short section.

## Capabilities

### New Capabilities

- `std_go_iplookup_family-match`: dual-tree family isolation; IPv4-mapped IPv6 classified with `To4()`.
- `std_go_iplookup_cidr-store`: AddCIDR, RemoveCIDR, Reset, Count; canonical prefix identity.
- `std_go_iplookup_match-label`: string stored on a prefix; Contains returns the winning longest-prefix label.

### Modified Capabilities

- None. `std_go_ci_test-suites` is unchanged: this package has no LIVE env and is not added to e2e or Pester jobs.

## Impact

- New `iplookup/` (stdlib only).
- `README.md` (`## Layout`, new section).
- `openspec/specs/domains.md` unchanged (`std` / `go` already allowed). `openspec/specs/map.md` regenerated after archive.
- `knowledge/devdocs/std_go_iplookup.md` and `index_std_go.md`.
- No change to `reclaim/`, `simpleredis/`, `windowcounter/`, `tokenbucket/`, `backendbackoff/`, or `.github/workflows/ci.yml` e2e jobs.
- Geoblock is not edited in this repo; a later geoblock change may import this module.
