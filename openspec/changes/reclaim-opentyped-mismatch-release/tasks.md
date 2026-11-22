# Tasks

## 1. Lease and exactly-once drop

- [ ] 1.1 Add `bindLease` and `dropOnce` per design.md Decisions 1-2. `dropWhenDone` keeps the `AfterFunc` stop on the lease and both watchers call `dropOnce`. The internal open path returns the lease; `OpenWithHooks` keeps its signature. Verify `go test -count=1 ./reclaim/` passes all existing tests, including Yaegi and race-sensitive repros.
- [ ] 1.2 `OpenTyped` releases the lease on a type mismatch and returns the unchanged error. Verify `go test -count=1 -run OpenTyped ./reclaim/`.

## 2. Tests

- [ ] 2.1 Add `reclaim/zzz_opentyped_mismatch_release_test.go` with cases (a)-(e) of Decision 5, helpers suffixed `MismatchRelease`. Case (a) must fail on the old code (Close never runs). Verify `go test -count=1 -race -run MismatchRelease ./reclaim/` (skip `-race` only if the toolchain has no cgo; say so).

## 3. Docs

- [ ] 3.1 One sentence in `knowledge/devdocs/std_go_reclaim.md` under OpenTyped: a type mismatch returns the error and does not keep the caller as a holder. Verify the sentence is present.

## 4. Local gate

- [ ] 4.1 `go vet ./...` and `go test -short -count=1 ./...`; then `go test -count=1 ./reclaim/`. Verify all exit 0.
