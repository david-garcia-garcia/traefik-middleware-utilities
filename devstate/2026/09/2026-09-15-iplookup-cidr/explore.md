# Explore
IssueKey: 2026-09-15-iplookup-cidr

## Concepts

```
  caller owns net.IP
          |
          v
     Helper.Contains -----------------+
          |                           |
          v                           v
     treeFor(To4()!=nil)        ipv6Tree
          |
          v
     ipv4Tree
          |
          v
     longest-prefix node --> metadata string
```

- **Helper**: in-process CIDR store in package `iplookup`. Same job as geoblock `IpLookupHelper` after PR #86 (two family trees), plus remove, per-prefix string, and a full clear. Not geoblock `file_monitor`, not `decide`.
- **Family tree**: one radix root for IPv4 (including IPv4-mapped IPv6 via `ip.To4() != nil`) and one for IPv6. A prefix never matches the other family.
- **Match label**: the string stored on the winning longest-prefix CIDR. Empty string is a stored value, not "no metadata".
- **Caller address**: this library does not reconstruct client IP, X-Forwarded-For, or Host. The caller passes `net.IP`.

Existing units: `reclaim.Table`, `backendbackoff.Gate`, `windowcounter.Limiter` — `New` + short type. No `iplookup/` on `origin/master`. Research: `knowledge/research/ext_geoblock_iplookup_cidr-family/`.

## Decisions

- Port dual-tree `treeFor` / `To4()` routing from geoblock PR #86 (`7f8e32d:pkg/iplookup/iplookup.go`). Keep `ipRadixTree` family-agnostic.
- House names: `iplookup.Helper`, `New()`, `AddCIDR`, `RemoveCIDR`, `Contains`, `Reset`, `Count`. Not `IpLookupHelper` / `IsContained` / `NewEmptyIpLookupHelper`.
- One lookup: `Contains(net.IP) (found bool, prefixLen int, metadata string, err error)`. Do not add a second metadata method.
- `AddCIDR(cidr, metadata string)`: same network+length overrides the label and does not bump `Count`.
- `RemoveCIDR` removes that stored prefix (canonical `ParseCIDR` network + mask length). Missing prefix → `false`, nil error. Parse failure → error.
- `Reset()` is production: drop both trees and `Count` to 0.
- Mutex on Helper: Traefik request path can `Contains` while a reload `AddCIDR` / `RemoveCIDR` / `Reset`.
- Specs under `std_go_iplookup_*` (this catalog, not geoblock `core_geoblock_*`).
- Proof: `go test -short ./iplookup/` plus in-package Yaegi like `backendbackoff`. No Pester this ticket (requirement out of scope).
- No usage Language write yet: the package is not on disk; packet `std_go_iplookup.md` after apply.

## Open questions

- Q: What are the exported names for the helper, constructors, remove, lookup, and reset?
  Rank: additive asked — new package this change creates; Desired names the port plus Remove, metadata, and Reset()
  Decision: assumed — `Helper`, `New()`, `AddCIDR(cidr, metadata string) error`, `RemoveCIDR(cidr string) (removed bool, err error)`, `Contains(ip net.IP) (found bool, prefixLen int, metadata string, err error)`, `Reset()`, `Count() int`. Skip geoblock `NewIpLookupHelper([]string)` (loop `AddCIDR` instead).
  By: explore

- Q: Does the match string belong only to the winning longest-prefix CIDR, or also to nodes walked on the way?
  Rank: additive asked — Desired says return the string when the IP matches that CIDR and keep longest-prefix semantics
  Decision: assumed — store the string on the prefix endpoint; `Contains` returns the winning prefix's string (empty if that prefix stored empty). Walked shorter prefixes are not returned.
  By: explore

- Q: Which OpenSpec ids host this package in this repo?
  Rank: additive asked — Desired is a new importable package; this catalog uses `std_go_<component>_<leaf>` (map.md has no iplookup family)
  Decision: assumed — three new leaves under family `std_go_iplookup`: `family-match` (dual-tree isolation), `cidr-store` (AddCIDR / RemoveCIDR / Reset / Count), `match-label` (string on store and Contains). Propose runs FindSpecHost before each folder write.
  By: explore

- Q: Who already owns the client address this helper looks up?
  Rank: additive asked — Contains takes an address; One job, one owner forbids reconstructing identity the host already chose
  Decision: assumed — none in this library. Callers pass `net.IP` they already selected (plugin / trusted hop). Helper MUST NOT read HTTP, X-Forwarded-For, or Host.
  By: explore

- Q: How should IPv4-mapped IPv6 addresses (`::ffff:a.b.c.d`) be classified?
  Rank: additive asked — Desired ports PR #86 `To4()` family routing
  Decision: assumed — `ip.To4() != nil` is IPv4 (same as geoblock PR #86). No third family.
  By: explore

- Q: Is `Reset` a test-only seam (like `reclaim.Table.Reset`) or a production clear?
  Rank: additive asked — Desired names `Reset()` that clears stored CIDRs
  Decision: assumed — production API on `Helper`. Name stays `Reset` because callers need a full clear on reload, not `ResetForTest`.
  By: explore

- Q: Should Helper serialize concurrent AddCIDR / RemoveCIDR / Reset / Contains?
  Rank: additive incidental — new unit this change creates; requirement does not name a lock
  Decision: assumed — yes, one mutex. Reload and request lookup can overlap in Traefik. Do not document lock-free use.
  By: explore
