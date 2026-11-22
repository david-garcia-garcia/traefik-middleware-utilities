## Motivation
Traefik operators wiring Redis rate limiting through `simpleredis.Config.Pass` sometimes see persistent WRONGPASS even when the password looks correct. Reports on https://github.com/traefik/traefik/issues/12078 trace that failure to Kubernetes Secret values that end with a literal newline: base64 `Cg==` is `\n`, not an empty suffix, so the engine receives a different password than the operator typed.

In this client, `New` already copies `cfg.Pass` without trimming and `authArgs` sends `[]byte(sr.pass)` on AUTH. That means `Pass` `"secret"` can succeed while `"secret\n"` is rejected as `redis:noauth`, matching the upstream symptom. The gap on master was proof and contract wording: there was no live test for the trailing-newline case, and the GoDoc on `Config.Pass` did not state byte-for-byte send or warn about Secret or file newlines. Without that, maintainers could add a silent trim “fix,” and operators would lack an in-repo anchor when debugging the same Secret mistake.

Leaving the gap alone does not change runtime behavior today, but it weakens the guardrail against credential transforms and leaves the traefik#12078 scenario undocumented beside the existing WRONGPASS live coverage (`"wrong-password"` only).

Priority: P3 — spec, docs, tests, or internal clarity with no new user or operator harm from this branch itself.

## Implementation
The binding keeps AUTH behavior unchanged and closes the gap on the `Config.Pass` documentation seam plus live proof on the requirepass engines.

The GoDoc on `Config.Pass` in `simpleredis/config.go` now states that AUTH receives the field byte for byte with no trimming, that a trailing newline from a Secret or file makes AUTH fail, and cites https://github.com/traefik/traefik/issues/12078. No edits to `New`, `authArgs`, `applyDefaults`, or `Config.User`.

A self-contained live file `simpleredis/traefik_issue12078_e2e_test.go` adds `TestTraefik12078_PasswordWithTrailingNewline`, copied from the reference repro. It reuses the package harness `runForEachLiveEngine` with `SIMPLEREDIS_LIVE_REDIS_AUTH` and `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` and `newTestRedis`. For each engine: control `Pass` `"secret"` must authenticate (Get succeeds or returns `redis:miss`); `Pass` `"secret\n"` must fail with `redis:noauth`. The test skips under `-short` or when those env vars are unset, same as other live AUTH tests in the package.

OpenSpec change metadata (`skip_specs: true`) records that the existing live AUTH and WRONGPASS catalog specs already cover the error mapping and this ticket adds no catalog delta.

## What this changes
**Operators.** When wiring `simpleredis.Config.Pass` from a Secret or file, ensure the value has no accidental trailing newline; the client sends the string exactly as stored and AUTH fails if an extra `\n` is present (see https://github.com/traefik/traefik/issues/12078).

**Admin users.** None.

**Developers.** `Config.Pass` GoDoc documents byte-for-byte AUTH, no trimming, and the newline failure mode with an upstream cite; `TestTraefik12078_PasswordWithTrailingNewline` locks that contract on the requirepass live engines without changing session code.

**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P3 — spec, docs, tests, or internal clarity with no new user or operator harm from this branch itself.
Reviewed head: 806e80fe
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-simpleredis-password-trailing-newline pushed | `git` |
| OpenSpec | simpleredis-password-trailing-newline | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115 | pr-host |
| CI | build 44996727348 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-simpleredis-password-trailing-newline on branch 2026-11-22-simpleredis-password-trailing-newline targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115; CI build 44996727348 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996727348.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_standards.md) — 2 total, 0 pending, 0 completed, 2 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-simpleredis-password-trailing-newline/devstate/2026/11/2026-11-22-simpleredis-password-trailing-newline/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 806e80fe78528efefc65f97982a95d35062e1732 | Card must match the branch you measured |

### Stored data model
None.
