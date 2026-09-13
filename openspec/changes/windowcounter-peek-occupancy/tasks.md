## 1. Occupancy lock test

- [ ] 1.1 Create `windowcounter/repro_peek_take_boundary_test.go` from the parent example: exact and buffered, fill `limit` Takes at a frozen clock, then Peek then Take. Keep `TestRepro_PeekAllowsWhenNextTakeDenies`.
- [ ] 1.2 Rewrite assertions to occupancy so the test PASSES on dest Peek: after N Takes, Peek allowed true / estimated N; next Take allowed false / estimated N+1. Do not require Peek allowed to equal Take allowed.

## 2. Docs only

- [ ] 2.1 Rewrite Peek godoc on `windowcounter/limiter.go`: occupancy (already-used estimate at or under limit), not “would the next Take admit.” Do not change Peek compare.
- [ ] 2.2 Update `knowledge/devdocs/std_go_windowcounter.md` How to use and Gotchas: Peek is occupancy; do not treat allowed as a reservation; at occupancy equal to limit Peek allows and the next Take denies and still increments.

## 3. Prove

- [ ] 3.1 Run `go test -short -count=1 -timeout 60s ./windowcounter` until passing, including the occupancy lock.
- [ ] 3.2 Run `openspec validate --change windowcounter-peek-occupancy --strict`
