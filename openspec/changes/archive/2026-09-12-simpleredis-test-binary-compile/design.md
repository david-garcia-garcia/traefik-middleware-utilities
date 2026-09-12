## Context

Dest `origin/master` already tracks `simpleredis/interpretedcost_test.go` and defines `writeGopathFile` in `simpleredis/yaegi_test.go`. `writeGopathClientprobe` delegates to it. See proposal.md for why that pairing must stay. Explore measured `go test -c ./simpleredis/` exit 0 and five `TestYaegi_*` PASS on this worktree.

## Goals / Non-Goals

**Goals:**
- Keep dest compile green.
- If dest compile is red, add the missing same-package helper with the signature the four call sites already use (`testing.TB`, GOPATH, package dir, file name, source).
- Leave dest’s tracked test files in place.

**Non-Goals:**
- Shared internal test-support across `tokenbucket`, `windowcounter`, and `reclaim`.
- `.gitignore` for `apm_modules/`.
- Copying the caller workspace’s untracked copies onto dest.

## Decisions

1. **Package-local helper, not a shared test package.** Dest already has the helper next to the Yaegi tests. A fourth copy in other packages is out of scope; extracting now would reshape four test trees for a compile that dest already has. Alternative: `internal/testgopath` — rejected (explore, out of scope).

2. **Do not replace dest files with caller untracked copies.** Content hashes differ; dest HEAD compiles. Alternative: merge caller `interpretedcost_test.go` — rejected (bound the ask; would risk a red binary).

3. **Implement is verify-then-edit.** First compile. Only if compile fails, add `writeGopathFile`. Do not edit `simpleredis.go`.

## Risks / Trade-offs

- [Sibling packages still duplicate the helper] → Mitigation: leave as out of scope; do not widen this change.
- [Caller dirty tree still fails locally] → Mitigation: this PR is dest; do not import that tree.
