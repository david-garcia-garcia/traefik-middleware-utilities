# Tasks

## 1. Store

- [x] 1.1 Add `iplookup/store.go` with the family store (host map + existing trie) and the label table (design.md Decisions 1-2). Verify `go build ./iplookup/` and `go vet ./iplookup/`.
- [x] 1.2 Wire `Helper` to the stores (`New`, `Reset`, `AddCIDR`, `RemoveCIDR`, `Contains`, `Count`) per Decisions 3-4. Public signatures unchanged. Verify `go test -count=1 ./iplookup/` passes every existing test, including the Yaegi tests.

## 2. Tests and benchmarks

- [x] 2.1 Add `iplookup/zzz_host_map_store_test.go` with cases (a)-(c) of Decision 6, helpers suffixed `HostMapStore`. Case (b) must fail on the old store. Verify `go test -count=1 -run HostMapStore ./iplookup/`.
- [x] 2.2 Extend the Yaegi test (own probe source, slug-suffixed) to cover IPv4 and IPv6 host add/replace/contains/remove/reset (Decision 5). Verify it passes under Yaegi and record the key type that shipped.
- [x] 2.3 Add load and Contains benchmarks, compiled and Yaegi; record dest vs new numbers for the delivery card.

## 3. Follow-up

- [x] 3.1 Write `knowledge/debt/<date>-iplookup-path-compress-prefix-trie.md` (note large: large lists of non-host prefixes still use one node per bit) and its `devstate/issues.md` row. Verify both exist.

## 4. Local gate

- [x] 4.1 `go vet ./...` and `go test -short -count=1 ./...`; then `go test -count=1 ./iplookup/`. Verify all exit 0.
