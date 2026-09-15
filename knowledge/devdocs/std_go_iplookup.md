# CIDR lookup

## Language

**Helper**:
An in-process CIDR store in package `iplookup`. IPv4 and IPv6 prefixes live on separate trees. Callers pass `net.IP` they already selected.
_Avoid_: reconstructing client address from HTTP or X-Forwarded-For; a shared radix root for both families; geoblock `IpLookupHelper` as the type name in this module

**Match label**:
The string stored on one CIDR prefix. `Contains` returns the winning longest-prefix label. Empty string is a stored value.
_Avoid_: attaching the label to every node walked; a second lookup method for metadata

**Family tree**:
One radix root for IPv4 (including IPv4-mapped IPv6 via `ip.To4() != nil`) and one for IPv6. A prefix cannot match the other family.
_Avoid_: classifying IPv4-mapped addresses as a third family

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`. The caller owns the address. `Reset` is a production clear for reload, not a test-only seam.

## How to use

- `New()` once. `AddCIDR(cidr, metadata)` per prefix. The same canonical network+length replaces the label and does not bump `Count`.
- `Contains(ip)` per request. Pass the `net.IP` the plugin already chose. On a miss, metadata is empty and prefixLen is 0.
- `RemoveCIDR` drops one prefix. Missing prefix returns false and a nil error.
- `Reset` clears both families. Safe to call more than once.
- Prove with `go test -short ./iplookup/...`. No live Redis/Dragonfly (see `knowledge/devdocs/std_go_test-suites.md`).

## Pattern snippet

```go
h := iplookup.New()
if err := h.AddCIDR("10.0.0.0/8", "office"); err != nil {
	return err
}
found, prefixLen, label, err := h.Contains(clientIP)
if err != nil {
	return err
}
_ = prefixLen
if !found {
	return errDenied
}
_ = label
```

## Key files

- `iplookup/helper.go` — `Helper`, `New`, `AddCIDR`, `RemoveCIDR`, `Contains`, `Reset`, `Count`
- `iplookup/tree.go` — family-agnostic radix insert, contains, remove
- `openspec/specs/std_go_iplookup_family-match/spec.md`, `openspec/specs/std_go_iplookup_cidr-store/spec.md`, `openspec/specs/std_go_iplookup_match-label/spec.md`

## Gotchas

- `1.2.3.4/32` does not match `102:304::1`. List both families when both should match.
- `10.0.0.1/8` and `10.0.0.0/8` are the same stored prefix after `ParseCIDR`.
- Traefik request lookup can overlap a reload: Helper serializes mutations with Contains.
- Do not import geoblock `pkg/iplookup`; this module is the reusable copy.
