## 1. Gate and credit

- [x] 1.1 Add `backendbackoff/` with `Config`, `New`, `Gate`, `Allow`, `Report`, `Close`, `SetNowForTest`, unexported `maxMemorySources` 65536, `dropExpired` / `dropOne` copied in shape from `tokenbucket/memory.go` (do not import tokenbucket)
- [x] 1.2 Apply zero-Config defaults and validation from `std_go_backendbackoff_allow`
- [x] 1.3 Implement saturating credit and CLOSED trip; Allow refreshes idle TTL including denies; Report has no context

## 2. Cooldown

- [x] 2.1 OPEN / HALF-OPEN, `BaseCooldown * 2^n` plus jitter capped at MaxCooldown, `Jitter` 0 disables jitter
- [x] 2.2 One outstanding probe, probe lease BaseCooldown, lost-probe re-Allow, probe success retains `n` and restores credit, probe failure increments `n`
- [x] 2.3 Reset `n` after one MaxCooldown of continuous CLOSED

## 3. Proofs

- [x] 3.1 Compiled tests: consecutive-failure trip, success credit, idle drop, deny refreshes TTL, canceled context, first-trip cooldown, probe retain/increment `n`, n reset, Close
- [x] 3.2 `gate_yaegi_test.go`: GOPATH interp, stdlib only, useunsafe false, Allow-then-Report trip
- [x] 3.3 `TestAlloc*` on warm CLOSED Allow; skip under race; ceilings from a measured Go 1.21 run with simpleredis slack convention
- [x] 3.4 `go test -short ./backendbackoff/...` passing; `go test -race -short ./backendbackoff/...` passing (skip alloc)

## 4. Docs and catalog

- [x] 4.1 README: new section, Layout row, one line in Why this exists so the module is not Redis-only
- [x] 4.2 Usage packet `knowledge/devdocs/std_go_backendbackoff.md` and `index_std_go.md` row
- [x] 4.3 `openspec validate backendbackoff-gate --type change --strict`
