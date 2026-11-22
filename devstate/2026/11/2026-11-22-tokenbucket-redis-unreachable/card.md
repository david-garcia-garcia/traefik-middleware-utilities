## Motivation
Aún no.

## Implementation
Aún no.

## What this changes
**Operators.** Ninguno.

**Admin users.** Ninguno.

**Developers.** Ninguno.

**End users.** Ninguno.

## Merge readiness
En curso. 0 ítems pendientes.

Priority: unknown — motivation not written
Reviewed head: 7ded9ad2
Owner decision: Obligatorio. Ver Explore Decisions.

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
| Branch | 2026-11-22-tokenbucket-redis-unreachable pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/114 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Ninguno.

## Deviations from the ask
Ninguna.

## Follow-up issues
Ninguno.

## How this fits together
Ticket 2026-11-22-tokenbucket-redis-unreachable en la rama homónima hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/114; CI no visto. Pedido TEST + DOC: cubrir Redis inalcanzable como `simpleredis.ErrUnreachable` (o `ErrTimeout`) con `allowed=false`, sin cambio de comportamiento. Medido en el worktree: `Redis.Allow` ya propaga el outage y deja la política al llamador; no hay defecto de política en esta librería. La existencia del defecto aguas arriba se tomó de https://github.com/traefik/traefik/issues/12091 (cerrado como duplicado de https://github.com/traefik/traefik/issues/12043; no hay PR upstream que cerrara #12091).

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Basta el GoDoc de Redis.Allow o hay que alinear el usage (Match Redis errors by Error() text) con errors.Is? | additive incidental — una línea de usage existente; el Design nombra GoDoc, no el usage. | assumed — alinear How to use a errors.Is / IsUnreachable contra ErrUnreachable y ErrTimeout, igual que std_go_simpleredis.md. No tocar std_go_windowcounter.md. | explore |
| ¿El spec vivo necesita un escenario que nombre ErrUnreachable / ErrTimeout? | additive incidental — aclaración de un contrato ya vivo; Affected lo deja a propose. | assumed — propose pliega en std_go_tokenbucket_allow errors.Is y Allowed false en Status cero; conservar «does not return allowed false as a substitute». | explore |
| ¿Hace falta un test del camino ErrTimeout? | additive incidental — el Design lo nombra en GoDoc; Tests solo nombra el puerto que rechaza. | assumed — no. GoDoc nombra ErrTimeout; el fichero 12091 cubre refuse. simpleredis ya clasifica timeout. | explore |
| ¿Este issue necesita un test live contra Redis/Dragonfly? | additive incidental — el repro de referencia es unitario contra 127.0.0.1:1. | assumed — no. TestLive_Limiter ya es el camino feliz. | explore |
| ¿Cómo adaptar la firma del repro (bool, wait, error) a (Status, error) de dest? | additive asked — Tests mapea TestTraefik12091_RedisUnreachable; dest cambió la firma en #106. | assumed — status, err := limiter.Allow(...); errors.Is(err, ErrUnreachable); fallar si status.Allowed es true. | explore |
| ¿Los tests existentes que casan Error() == RedisUnreachable se reescriben a errors.Is? | additive incidental — 2 sitios enumerados; ningún criterio pide reescribirlos. | assumed — no reescribir. El fichero 12091 es la prueba errors.Is. | explore |

## Findings
Ninguno.

## Axis review
Ninguno.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 7ded9ad2fed1e3621ef2398c5a8bc13adb63bd6f | Card must match the branch you measured |

### Stored data model
Ninguno.
