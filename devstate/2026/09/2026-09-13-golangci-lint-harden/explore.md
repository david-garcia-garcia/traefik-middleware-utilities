# Explore
IssueKey: 2026-09-13-golangci-lint-harden

Measured on dest `origin/master` `2847a81` with golangci-lint **v1.63.4**, issues uncapped (`max-issues-per-linter: 0`, `max-same-issues: 0`). Requester counts were from another branch; dest wins.

## Concepts

- **Dest lint** — `.golangci.yml` enables ten linters only. `.github/workflows/ci.yml` job `lint` uses `golangci/golangci-lint-action` `version: latest`. Spec `openspec/specs/std_go_ci_test-suites/spec.md` requires a `lint` job, not a pin. Usage `knowledge/devdocs/std_go_test-suites.md` points at those two files.
- **gocritic `enabled-checks`** — adds to the default checker set. Full dest output with `unnamedResult` added: **20** (`unnamedResult` 17 + default `assignOp` 3 in `tokenbucket/clock.go`). No other default-checker hits. `borrow` / `dial` are **not** in the 17.
- **`unnamedResult.checkExported`** — dest measured: `false` → 17 hits; `true` → 3 hits (exported `windowcounter` Take/Peek/…). `true` means exported-only and is weaker.
- **errorlint on dest** — **10** hits, all `==` / `!=` on errors. The requester’s 11th (type assert on `handshakeFailure` in `commands_exec.go`) is `not found` (PR #67). Remaining: `commands_exec.go:197` `err == errTimeout`, `:202` `err == errUnreachable`; `resp.go:124` / `:153` `bulkErr == errMiss`; `:176` `err == errIssue || err == errUnsupportedReply`; `:211` `err == bufio.ErrBufferFull`; `resp_test.go:451,465,558,573` `err != errIssue`.
- **Identity retry classifier** — `errUnreachable` is `ErrUnreachable`. `ErrPoolWait` and `errNotFromNew` are `fmt.Errorf("%w", ErrUnreachable)`. `shouldRetry` uses `err == errUnreachable` so those wraps are not retried. Spec `openspec/specs/std_go_simpleredis_tcp-session/spec.md` Requirement “Full pool wait”: identity compare (or a pool-wait check before unreachable) is required. Usage `knowledge/devdocs/std_go_simpleredis.md` Gotchas: do not rewrite retry onto `errors.Is(err, ErrUnreachable)` alone.
- **Yaegi `errors.As` panic** — reason 1 in the ask. Dest has **no** remaining type-assert-on-error site. `handshakeFailure` is gone. Remaining errorlint sites are `==` on sentinels, not `errors.As`. Do not attach this reason to those lines.
- **forcetypeassert** — **14** hits, all `_test.go` (Yaegi `Eval` asserts plus `reclaim/table_test.go` `value.(*box)`). Enable and exclude `_test.go` so production stays a zero-hit ratchet.
- **testpackage** — **56** hits. Tests are in-package white-box (`sr.inUseTurns`, `sr.borrow`, `sr.idleConns`). Leave disabled.
- **Windows `diff`** — `goimports` / `gofumpt` fail locally: `exec: "diff": executable file not found`. `gofmt` is already enabled and does not need `diff`.
- **in-flight SimpleRedis PRs** — same files as this change’s `borrow` / `dial` names and `errorlint` `==` sites.

```
Dest lint (2847a81, v1.63.4, uncapped)
  17 zero linters          0
  gocritic + unnamedResult  20  (17 unnamedResult + 3 assignOp)
  thelper                  15
  revive (golangci default) 8
  errorlint                10
  forcetypeassert          14  (all _test.go)
  dupword                  1
  prealloc                 1
  stylecheck               0
  errname                  0
  testpackage              56  (stay off)
  goimports/gofumpt        cannot run on this Windows host
```

## Decisions

- Enable the 17 zero-hit linters. None grew a hit on dest. No real-bug stop.
- Enable `gocritic` with `unnamedResult` **added** (do not replace defaults). `checkExported: false`. Comment in `.golangci.yml` that `true` is exported-only and weaker. Name every `unnamedResult` result; keep explicit `return` (`nakedret` stays on). Name `borrow` / `dial` by hand so `handshakeFailed` is in the signature. Do not reorder `error` last (revive `error-return` will still fire → documented `nolint:revive` on those two funcs; deferred reorder in the PR body).
- Enable `thelper`, `revive`, `dupword`, `prealloc`, `stylecheck`, `errname`. Fix dest hits (not the requester’s off-dest mix). `stylecheck` / `errname` are a zero-hit ratchet on dest (reclaim has no package comment, but golangci’s default stylecheck/revive do not flag ST1000 / `package-comments` here — do not add extra revive rules to chase that).
- Enable `forcetypeassert` with `issues.exclude-rules` path `_test.go`. Enable `errorlint`. Convert **none** of the 10 `==` sites to `errors.Is` (see open question). Type-assert site gone.
- Do not enable `testpackage`. Record the rejection in `knowledge/devdocs/std_go_test-suites.md` Gotchas.
- Leave `goimports` / `gofumpt` out. Pin CI to `v1.63.4`. `issues.exclude-dirs`: `apm_modules`, `.agents`.
- No runtime behavior change. Merge after the five in-flight SimpleRedis branches.

## Open questions

- Q: Which golangci-lint v1.x does CI pin?
  Rank: additive asked — Desired #10 names pin to the exact v1.x this run validates; v2 schema is out
  Decision: resolved — pin `v1.63.4` (this host’s `golangci-lint version` and every dest count above).
  By: explore

- Q: goimports / gofumpt: enable in CI, or leave out?
  Rank: additive asked — Desired #9 names either
  Decision: resolved — leave out. This Windows host cannot run them (`diff` missing). One `.golangci.yml` cannot be CI-only without breaking `golangci-lint run` for Windows contributors. `gofmt` already covers format.
  By: explore

- Q: Convert any dest errorlint `==` site to `errors.Is`, or nolint all ten?
  Rank: bounded asked — Desired #6 names convert only when neither listed reason applies; dest has 10 sites, enumerated in Concepts
  Decision: resolved — convert none; `//nolint:errorlint` on all ten. `err == errUnreachable` is the tcp-session identity requirement (pool wait / not-from-New wrap Unreachable). The other nine are identity classifiers on sentinels this package (or `bufio.ReadSlice`) returns as the exact value: timeout not-retry, decode `errMiss` → nil slot, dirty-protocol `errIssue` / `errUnsupportedReply`, buffer-full → `errIssue`, tests that lock decode returns `errIssue` itself. Reason 1 (Yaegi `errors.As` panic on a package-local struct) does not apply on dest — that type is gone. Do not write a fake Yaegi reason on `==` lines.
  By: explore

- Q: Fold the lint pin into `std_go_ci_test-suites`, or add a new spec leaf?
  Rank: additive asked — Desired / Affected names possibly that spec and `std_go_test-suites.md`
  Decision: resolved — fold. FindSpecHost: `std_go_ci_test-suites` (high). Usage gotchas in `std_go_test-suites.md`.
  By: propose

- Q: Enable extra revive `package-comments` / stylecheck ST1000 to force a reclaim package comment the requester expected?
  Rank: additive incidental — dest default revive/stylecheck are 0 for that; Desired says fix measured hits, not extra rules
  Decision: resolved — do not add those rules. Bound the ask. `stylecheck` stays a zero-hit ratchet.
  By: explore
