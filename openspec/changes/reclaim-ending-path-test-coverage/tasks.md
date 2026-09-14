## 1. Land the gap tests

- [x] 1.1 Copy `D:/repositories/traefik-middleware-utilities/reclaim/table_gaps_test.go` verbatim into `reclaim/table_gaps_test.go` in this worktree
- [x] 1.2 Confirm `git diff origin/master -- reclaim/table.go` is empty

## 2. Prove coverage and race

- [x] 2.1 Run `go test -count=1 -timeout 10m ./reclaim/` and confirm green
- [x] 2.2 Run `docker run --rm -v "<worktree>:/src" -w /src golang:1.25 go test -race -count=1 -timeout 10m ./reclaim/` and confirm green
- [x] 2.3 Run `go test -count=1 -covermode=atomic -coverprofile=<tmp>.cover -timeout 10m ./reclaim/` then `go tool cover -func=<tmp>.cover`; confirm 100.0% of statements and zero profile rows whose third column is 0

## 3. Spec deltas already in this change

- [x] 3.1 Leave `openspec/changes/reclaim-ending-path-test-coverage/specs/std_go_reclaim_value-lifecycle/spec.md` and `.../std_go_reclaim_context-lease/spec.md` as the Reset Sleep-panic and Reset-unmap scenarios; do not add a third spec folder
