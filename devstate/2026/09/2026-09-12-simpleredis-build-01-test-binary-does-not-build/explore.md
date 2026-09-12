# Explore
IssueKey: 2026-09-12-simpleredis-build-01-test-binary-does-not-build

## Concepts

**SimpleRedis test binary** — one `go test ./simpleredis/` compile. A missing helper in any `_test.go` fails the whole package, so no unit, Yaegi, or alloc-guard test runs.

**writeGopathFile** — same-package test helper in `simpleredis/yaegi_test.go`. Writes one source file under `GOPATH/src/<pkg>/<name>` for Yaegi to import. `writeGopathClientprobe` delegates to it. `interpretedcost_test.go` calls it at four sites (`:21`, `:190`, `:273`, `:380`).

**Dest vs caller dirty tree** — `origin/master` already tracks `interpretedcost_test.go`, `bench_test.go`, and `writeGopathFile`. The ticket’s compile failure was measured on the caller workspace’s untracked copies at an older HEAD. This run’s worktree is dest.

**Yaegi GOPATH layout** — Traefik/Yaegi load from GOPATH, not modules (`knowledge/research/ext_traefik_plugins_local-loader/`). Test helpers materialise that tree; they are not production SimpleRedis.

```
interpretedcost_test.go  --calls-->  writeGopathFile (yaegi_test.go)
                                         |
                                         +-- writeGopathClientprobe
                                         +-- (siblings in tokenbucket/windowcounter/reclaim, not shared)
```

## Decisions

- Bound this run to dest. Do not copy or merge the caller’s untracked `interpretedcost_test.go` / `bench_test.go` / `yaegi_test.go`.
- Do not add a second `writeGopathFile`. Dest already has the signature the four call sites use (`testing.TB`, `goPath`, `pkg`, `name`, `src`).
- Do not extract a shared internal test-support package. Out of scope.
- Do not gitignore `apm_modules/`. That is ci-01.
- Propose records the compile invariant on the existing SimpleRedis spec family (`std_go_simpleredis_resp-commands`): `go test ./simpleredis/` SHALL compile because `writeGopathFile` lives in `yaegi_test.go` and `interpretedcost_test.go` is in that package. No new spec leaf. No production `simpleredis.go` change unless compile fails on dest (it does not).
- Prove: `go test -c ./simpleredis/` and `go test -short -count=1 -run ^TestYaegi_ ./simpleredis/`. Measured on dest worktree: compile exit 0; five cases PASS (`TestYaegi_NewGetSetDel`, `TestYaegi_IncrAndEval`, `TestYaegi_EvalNoScriptFallback`, `TestYaegi_MSetEXNative`, `TestYaegi_MSetEXLua`).

## Open questions

- Q: Dest already has `writeGopathFile` and a compiling test binary. Does this run still add a product helper, or only lock the dest proof in spec?
  Rank: additive asked — Desired 1–3 name dest compile and the helper; dest already has both
  Decision: resolved — dest `simpleredis/yaegi_test.go:138` `writeGopathFile` satisfies the four `interpretedcost_test.go` sites. `go test -c -o NUL ./simpleredis/` exit 0. Implement does not add another helper and does not replace dest files. Propose deltas `std_go_simpleredis_resp-commands` with a compile-invariant requirement.
  By: explore

- Q: Should `writeGopathFile` be factored into a shared internal test-support package used by simpleredis, tokenbucket, windowcounter, and reclaim?
  Rank: additive incidental — Out of scope lists this; no shared package exists to reshape
  Decision: resolved — keep the dest package-local helper. Sibling copies stay.
  By: explore

- Q: Should this change add `apm_modules/` to `.gitignore`?
  Rank: additive incidental — Out of scope; ticket names ci-01
  Decision: resolved — not this change.
  By: explore

- Q: Why do the caller’s untracked copies diverge from dest?
  Rank: additive incidental — Unknowns; not needed to keep dest compiling
  Decision: assumed — caller HEAD was behind `origin/master`; ignore those copies.
  By: explore
