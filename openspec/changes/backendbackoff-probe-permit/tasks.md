# Tasks

## 1. Generation and permit

- [ ] 1.1 Add `seq uint64` to `Gate`, `gen uint64` to `memEntry`, and `type Permit struct { key string; gen uint64 }` with `Allowed() bool` in `backendbackoff`. Set `gen = ++g.seq` on entry creation and on every state transition named in design.md Decision 1. Verify `go build ./backendbackoff/` and `go vet ./backendbackoff/`.
- [ ] 1.2 Change `Allow(key string) (Permit, time.Duration, error)`: an admit returns `Permit{key, entry.gen}` (after any transition bump), a deny returns the zero `Permit`. Verify existing deny/retryAfter tests pass after migration (task 2.1).
- [ ] 1.3 Change `Report(p Permit, success bool) error` with the check order of design.md Decision 3. A stale or zero permit is a no-op without TTL refresh. Verify `go test -short -count=1 ./backendbackoff/`.

## 2. Callers and tests

- [ ] 2.1 Migrate every in-tree `Allow`/`Report` call (tests, benches, the Yaegi probe source in `zzz_gate_yaegi_test.go`). No legacy wrappers. Verify `go test -count=1 ./backendbackoff/` (not `-short`) passes, including Yaegi and `TestAllocAllowWarm`.
- [ ] 2.2 Add `backendbackoff/zzz_probe_permit_test.go` with the five cases of design.md Decision 5, helpers suffixed `ProbePermit`. Case (a) must fail on the old code shape (stale success closing the gate). Verify `go test -count=1 -run ProbePermit ./backendbackoff/`.

## 3. Docs

- [ ] 3.1 Update `README.md` snippet and `knowledge/devdocs/std_go_backendbackoff.md`: `Allow` returns a `Permit`, `Report` takes it, a report from another generation is ignored. Verify both files show `Report(p, ` / `Permit` and no `Report(key,` remains.

## 4. Local gate

- [ ] 4.1 `go vet ./...` and `go test -short -count=1 ./...` from the worktree; then `go test -count=1 ./backendbackoff/`. Verify all exit 0.
