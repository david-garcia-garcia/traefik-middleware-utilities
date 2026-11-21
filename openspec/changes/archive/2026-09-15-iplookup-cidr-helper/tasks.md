## 1. Helper and family trees

- [x] 1.1 Add `iplookup/` with `Helper`, `New`, dual `ipRadixTree`, `treeFor` via `To4()`, mutex, `Count`
- [x] 1.2 Port insert/contains bit walking from geoblock PR #86 (IPv4 bitStart 96, IPv6 bitStart 0) onto separate trees
- [x] 1.3 Implement `AddCIDR(cidr, metadata)` with canonical ParseCIDR identity; override metadata without bumping Count

## 2. Remove, reset, label

- [x] 2.1 `RemoveCIDR`: parse, clear endpoint, prune empty nodes; missing prefix returns false, nil error
- [x] 2.2 `Reset` replaces both trees and zeros Count
- [x] 2.3 `Contains` returns winning longest-prefix metadata (empty string on miss or stored empty)

## 3. Proofs

- [x] 3.1 Compiled tests: family collisions (`1.2.3.4/32` vs `102:304::1`, `808:808::/32` vs `8.8.8.8`), IPv4-mapped hit, same-prefix override, overlapping remove, missing remove, Reset both families, longest-prefix label
- [x] 3.2 `helper_yaegi_test.go`: GOPATH interp, stdlib only, useunsafe false, AddCIDR then Contains hit and cross-family miss
- [x] 3.3 `go test -short ./iplookup/...` passing; `go test -race -short ./iplookup/...` passing

## 4. Docs and catalog

- [x] 4.1 README: new section and Layout row
- [x] 4.2 Usage packet `knowledge/devdocs/std_go_iplookup.md` and `index_std_go.md` row
- [x] 4.3 `openspec validate iplookup-cidr-helper --type change --strict`
