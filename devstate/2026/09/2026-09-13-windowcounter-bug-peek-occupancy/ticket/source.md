After exactly limit Takes, Peek allowed=true (est=limit) and the next Take denies (est=limit+1). THIS IS EXPECTED OCCUPANCY. Do NOT change Peek to next-hit (occupancy+1). Fix is DOCUMENTATION + a test that locks occupancy.

Agreed how: Document in Peek godoc, knowledge/devdocs/std_go_windowcounter.md (usage + Gotchas), and the spec: Peek is already-used, not “would the next Take admit.” Scenario “Take's allowed matches Peek's allowed” applies when the increment does not cross limit. At exactly limit they may differ. Rewrite TestRepro_PeekAllowsWhenNextTakeDenies to LOCK occupancy: after N Takes, Peek allowed=true/est=N, next Take allowed=false/est=N+1 (test must PASS, not fail).

Implement order (required):
1. CREATE tests first from example `d:\repositories\traefik-middleware-utilities\windowcounter\repro_peek_take_boundary_test.go` but rewrite assertions to the agreed occupancy contract so they pass on current code (this is the lock, not a red-fail expecting allowed to match).
2. Then docs/spec only. Do not change Peek compare.
3. Confirm `go test -short -count=1 -timeout 60s ./windowcounter` passes including the occupancy lock test.

Bound the ask: only this bug.
