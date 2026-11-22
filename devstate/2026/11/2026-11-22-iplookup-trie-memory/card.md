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
En curso. Quedan 0 elementos.

Priority: unknown — motivation not written
Reviewed head: 303696bd
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | No listo |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | PR remoto — la prueba de CI cubre esto |
| Review resolution | 6/6 | sin comentarios abiertos en el PR |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-22-iplookup-trie-memory pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/119 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Ninguno.

## Deviations from the ask
Ninguno.

## Follow-up issues
Ninguno.

## How this fits together
Ticket 2026-11-22-iplookup-trie-memory en la rama 2026-11-22-iplookup-trie-memory hacia master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/119; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Qué representación segura para Yaegi puede bajar memoria y punteros de GC ≥10× en listas grandes de hosts aleatorios (IPv4 /32, IPv6 /128) sin romper los contratos vivos? | structural asked — reescritura del almacén radix existente (`ipRadixTree` / un `radixNode` por bit); Desired nombra bajar memoria y punteros de GC al menos un orden de magnitud manteniendo cada contrato actual | assumed — defer pick to propose (tech lead). OPTIONS, no Chosen: (1) Patricia / path compression — un nodo por bit hoy vs ~2N; IPv6 decenas de ×, IPv4 ~6× y sigue siendo grafo de punteros, ≥10× de memoria IPv4 no está garantizado. (2) Rangos ordenados empaquetados — ~8–32 B/entrada, sin grafo de punteros por nodo; LPM solapado y RemoveCIDR piden identidad extra. (3) Otras — hash de hosts más lista de prefijos, ART/stride. Ninguna Chosen aquí. | explore |
| Cuánto cuestan la carga y Contains en Yaegi de una representación nueva? | additive asked — medición de una representación que este cambio introduciría; Desired nombra medir compilado y Yaegi (tiempo de carga y Contains) | assumed — este reproduce y la tabla de caza son solo compilados. No afirmar una victoria Yaegi a partir de 89 ms / 1.072 s / 291 ms de carga ni de 106 ns / 51 ns de Contains. Medir los dos motores cuando propose elija una forma. | explore |

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
| Reviewed head | 303696bd23481cb183834b0388a7fe837f406d78 | Card must match the branch you measured |

### Stored data model
Ninguno.
