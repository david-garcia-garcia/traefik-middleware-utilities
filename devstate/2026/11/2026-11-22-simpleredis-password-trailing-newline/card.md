The existence of this defect was taken from https://github.com/traefik/traefik/issues/12078 (cause identified in comments: Kubernetes Secret values ending in a newline).

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
In progress. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: 3bd8cd17
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Not ready |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-simpleredis-password-trailing-newline pushed | `git` |
| OpenSpec | simpleredis-password-trailing-newline | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-22-simpleredis-password-trailing-newline on branch 2026-11-22-simpleredis-password-trailing-newline targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/115; CI not seen. Upstream: https://github.com/traefik/traefik/issues/12078 (no upstream PR).

## Explore Decisions
- Q: ¿Un Secret o fichero con `\r\n` (Windows) debe cubrirse en el test?
  Rank: additive incidental — casos extra; Out of scope: whitespace distinto del `\n` del test de referencia; Tests nombra `"secret\n"`
  Decision: assumed — solo `"secret\n"` como el repro de referencia.
- Q: ¿Qué hace Traefik / go-redis con el mismo Secret?
  Rank: additive incidental — investigación de terceros extra; Out of scope: cambiar AUTH en Redis/Dragonfly o Traefik upstream
  Decision: assumed — fuera de este cliente; citar https://github.com/traefik/traefik/issues/12078; no portar trim de Traefik ni go-redis.
- Q: ¿El paquete de uso `std_go_simpleredis.md` debe ganar un Gotcha de newline, o basta el GoDoc de `Config.Pass`?
  Rank: additive incidental — edición extra de usage; Design nombra GoDoc en `Config.Pass`
  Decision: assumed — DOC requerida = GoDoc de `Config.Pass` (byte a byte, sin recorte, newline de Secret/fichero hace fallar AUTH, cita #12078). El usage packet no es la superficie nombrada.
- Q: ¿Se añade una fila fake `"secret\n"` en `TestAuthArgumentForms` para el job Unit `-short`?
  Rank: additive incidental — caso unit extra; Tests nombra solo la copia live
  Decision: assumed — solo el e2e live copiado; Unit `-short` se queda como hoy. El envío es `[]byte(sr.pass)` sin trim.
- Q: ¿El GoDoc de `Config.User` también debe decir que no se recorta?
  Rank: additive incidental — doc de otro campo; Design nombra `Config.Pass`; Out of scope recortar `User`
  Decision: assumed — GoDoc solo en `Config.Pass`; `User` se queda como está.

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 3bd8cd1793173474797eb73c8797e33ec78b9bff | Card must match the branch you measured |

### Stored data model
None.
