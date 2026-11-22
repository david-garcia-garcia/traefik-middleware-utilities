## Motivation
Not yet.

## Implementation
Not yet.

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: 319fbdeb
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-tokenbucket-expired-bucket-reclaim pushed | `git` |
| OpenSpec | tokenbucket-expired-bucket-reclaim | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111 | pr-host |
| CI | build 44996123943 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_tokenbucket_allow](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-tokenbucket-expired-bucket-reclaim/openspec/changes/tokenbucket-expired-bucket-reclaim/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-tokenbucket-expired-bucket-reclaim on branch 2026-11-22-tokenbucket-expired-bucket-reclaim targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/111; CI build 44996123943 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44996123943.

Tests de regresión: diez fuentes ociosas más una nueva dejan un bucket en Memory; tras Allow, Redis y Dragonfly dejan PTTL en (0, ttl]. Sin cambio de producto. Origen del defecto (Traefik ttlmap, no este repositorio): https://github.com/traefik/traefik/issues/13704

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Cómo adaptar los tests de referencia a `Allow(ctx, key) (Status, error)` sin copiar helpers de otros issues? | additive asked — ficheros de test que este cambio crea; la línea Tests del Design nombra ficheros autocontenidos con sufijo 13704 | assumed — llamar `Allow` como `(status, err)` y fallar si `err != nil`. El test Memory no usa `countAdmits`; cuenta `len(limiter.buckets)` bajo el mutex. El e2e declara `liveKey13704` y `evalKey13704` en `traefik_issue13704_e2e_test.go` (Eval `pttl` con `ScriptSHA1Hex`, KEYS[1] para Dragonfly). No copiar `liveKey`/`evalKey` del árbol de repros ni de otros issues. | propose |


## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 319fbdeb271ffa603d83d8b9bb62ccedab6c1d31 | Card must match the branch you measured |

### Stored data model
None.
