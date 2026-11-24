# Tasks

## 1. API

- [x] 1.1 Add `probe bool` to `Permit` and `Permit.Probe()`. Set it on the two HALF-OPEN admits in `Allow`. Verify `go build ./backendbackoff/` and `go vet ./backendbackoff/`.
- [x] 1.2 Add `Gate.Tripped(key)`: read-only, idle-expired keys are not tripped, `errClosed` after Close.

## 2. Tests

- [x] 2.1 `backendbackoff/zzz_tripped_test.go`: Tripped across trip, HALF-OPEN, probe success and probe failure; Tripped does not refresh the idle deadline; Close; which admits are probes.
- [x] 2.2 `TestYaegi_TrippedThenProbe`: trip, Tripped, probe permit, close, under Yaegi with stdlib only.

## 3. Docs

- [x] 3.1 `README.md` and `knowledge/devdocs/std_go_backendbackoff.md` describe `Probe` and `Tripped`.

## 4. Local gate

- [x] 4.1 `go vet ./...`, `go test -short -count=1 ./...`, `go test -count=1 ./backendbackoff/`, `golangci-lint run ./backendbackoff/...`.
