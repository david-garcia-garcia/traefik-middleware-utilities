# Requirement
IssueKey: 2026-09-15-iplookup-cidr

## Problem
Middlewares need a shared, Yaegi-safe CIDR lookup helper. traefik-geoblock PR #86 fixed IPv4/IPv6 cross-family false matches, but that code lives in geoblock, not in this utilities module. Callers also need remove, per-CIDR metadata, and a full clear — not present on the PR #86 head.

## Current (code)
- `iplookup/` package: not found on `origin/master` (this repo has `reclaim/`, `simpleredis/`, `tokenbucket/`, `windowcounter/`, `backendbackoff/` only).
- CIDR radix lookup in geoblock PR #86: `david-garcia-garcia/traefik-geoblock@7f8e32d:pkg/iplookup/iplookup.go` — dual trees, `AddCIDR`, `IsContained`, no remove/metadata/reset.

## Desired
- Port the PR #86 `IpLookupHelper` behavior (separate IPv4/IPv6 trees; `To4()` family routing) into import path `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`.
- (A) Remove a previously stored CIDR from the helper.
- (B) Associate a string metadata value when storing a CIDR; return that string on lookup when the IP matches that CIDR (longest-prefix semantics unchanged). Storing the same CIDR again replaces metadata.
- `Reset()` clears all stored CIDRs (and associated metadata).
- Unit tests covering family isolation (from geoblock PR #86) plus remove, metadata override, and reset.

## Affected
- New `iplookup/` package and tests (pattern: `reclaim/` top-level package).
- `go.mod` consumers; geoblock can later depend on this module instead of `pkg/iplookup` (follow-up outside this ticket text).

## Out of scope
- Porting `pkg/iplookup/file_monitor.go` or geoblock plugin wiring.
- Changing geoblock `decide` or allow/block list config UX.
- Pester/Yaegi e2e probe unless a later phase requires it (not named in the local spec).

## Unknowns
- Exact exported names for remove/metadata APIs (e.g. `RemoveCIDR` vs `DeleteCIDR`; whether `IsContained` grows a metadata out-param or a new result type).
- Whether metadata attaches to the CIDR endpoint only or also surfaces on partial-path matches during longest-prefix walk (ticket implies match-time string for the winning CIDR).
- OpenSpec spec id naming in this repo (`std_go_iplookup_*` vs other prefix).

## Tensions
None.
