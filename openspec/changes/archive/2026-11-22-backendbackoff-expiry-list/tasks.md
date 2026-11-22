# Tasks

## 1. Expiry list

- [x] 1.1 Add `key`, `prev`, `next` to `memEntry` and `head`, `tail` to `Gate`; add the list helpers of design.md Decision 1 (concrete pointers, no `container/list`). Verify `go build ./backendbackoff/` and `go vet ./backendbackoff/`.
- [x] 1.2 Rewrite `loadEntry` per Decision 3 and relink after every `expireAt` write in `Allow` and `Report` (Decision 2). Remove the full-map walks of `dropExpired`/`dropOne`. `Close` resets head/tail. Verify `go test -count=1 ./backendbackoff/` passes unchanged tests, including Yaegi and `TestAllocAllowWarm`.

## 2. Tests and benchmarks

- [x] 2.1 Add `backendbackoff/zzz_expiry_list_test.go` with cases (a)-(d) of Decision 5, helpers suffixed `ExpiryList`, O(n) prefill. Verify `go test -count=1 -run ExpiryList ./backendbackoff/`.
- [x] 2.2 Add compiled and Yaegi new-key-at-full-table benchmarks. Run them and record the numbers (compiled and Yaegi) for the delivery card. Verify the compiled per-op cost at 65,536 keys is within a small constant of the cost at 0 keys (no linear growth).
  - compiled `BenchmarkAllow/new-key-empty`: 215.8 ns/op, 183 B/op, 3 allocs/op (Intel Core Ultra 7 265K)
  - compiled `BenchmarkAllow/new-key-full-table`: 183.1 ns/op, 182 B/op, 2 allocs/op (explore baseline 1.8 ms at 65,536)
  - Yaegi `BenchmarkYaegiAllow/new-key-full-table`: 14479 ns/op (explore baseline 5.1 ms Yaegi at 16,384)

## 3. Debt

- [x] 3.1 Delete `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md` and close its row if `devstate/issues.md` lists it. Verify the file is gone.

## 4. Local gate

- [x] 4.1 `go vet ./...` and `go test -short -count=1 ./...`; then `go test -count=1 ./backendbackoff/`. Verify all exit 0.
