## 1. Helper and family trees

- [ ] 1.1 Add `iplookup/` with `Helper`, `New`, dual `ipRadixTree`, `treeFor` via `To4()`, mutex, `Count`
- [ ] 1.2 Port insert/contains bit walking from geoblock PR #86 (IPv4 bitStart 96, IPv6 bitStart 0) onto separate trees
- [ ] 1.3 Implement `AddCIDR(cidr, metadata)` with canonical ParseCIDR identity; override metadata without bumping Count

## 2. Remove, reset, label

- [ ] 2.1 `RemoveCIDR`: parse, clear endpoint, prune empty nodes; missing prefix returns false, nil error
- [ ] 2.2 `Reset` replaces both trees and zeros Count
- [ ] 2.3 `Contains` returns winning longest-prefix metadata (empty string on miss or stored empty)

## 3. Proofs

- [ ] 3.1 Compiled tests: family collisions (`1.2.3.4/32` vs `102:304::1`, `808:808::/32` vs `8.8.8.8`), IPv4-mapped hit, same-prefix override, overlapping remove, missing remove, Reset both families, longest-prefix label
- [ ] 3.2 `helper_yaegi_test.go`: GOPATH interp, stdlib only, useunsafe false, AddCIDR then Contains hit and cross-family miss
- [ ] 3.3 `go test -short ./iplookup/...` passing; `go test -race -short ./iplookup/...` passing

## 4. Docs and catalog

- [ ] 4.1 README: new section and Layout row
- [ ] 4.2 Usage packet `knowledge/devdocs/std_go_iplookup.md` and `index_std_go.md` row
- [ ] 4.3 `openspec validate iplookup-cidr-helper --type change --strict`
