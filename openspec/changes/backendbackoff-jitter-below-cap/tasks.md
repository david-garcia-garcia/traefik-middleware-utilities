# Tasks

## 1. Formula

- [ ] 1.1 Rewrite the jitter part of `cooldownDuration` in `backendbackoff/gate.go` per design.md Decision 1. Do not touch `allow.go`. Verify `go test -count=1 ./backendbackoff/` passes the existing tests (they use `Jitter: 0`) including Yaegi.

## 2. Tests

- [ ] 2.1 Add `backendbackoff/zzz_jitter_below_cap_test.go` with cases (a)-(d) of Decision 3, helpers suffixed `JitterBelowCap`. Case (a) must fail against the old clamp (50 % at Max). Verify `go test -count=1 -run JitterBelowCap ./backendbackoff/`.

## 3. Docs

- [ ] 3.1 Update the code comment on `cooldownDuration` to describe the interval. Verify the comment no longer says the result is capped by clamping.

## 4. Local gate

- [ ] 4.1 `go vet ./...` and `go test -short -count=1 ./...`; then `go test -count=1 ./backendbackoff/`. Verify all exit 0.
