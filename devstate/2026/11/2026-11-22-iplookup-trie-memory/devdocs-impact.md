# Devdocs impact
change: iplookup-host-map-store

## Units
- CIDR lookup — subsystem — `iplookup/helper.go` (family stores in `iplookup/store.go`, label table in `iplookup/labeltable.go`)

## Findings
- [x] stale-usage  CIDR lookup — `std_go_iplookup` Language still says "separate trees" / "Family tree" as one radix root; Key files list `tree.go` only; Gotcha still says "v4 tree"
