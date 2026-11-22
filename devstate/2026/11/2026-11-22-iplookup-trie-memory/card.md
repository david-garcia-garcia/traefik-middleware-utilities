## Motivation
Helper guarda prefijos CIDR en un trie por familia. Quien llama (un plugin con blocklist o feed de reputación) inserta cada red; `insert` recorre el prefijo bit a bit y asigna un `radixNode` (~48 B: bool, int, string, dos punteros) por bit que no se comparte. Prefijos de host aleatorios solo comparten unos ~log2(N) niveles altos, así que N entradas crean cerca de N × (bits − log2 N) nodos: hasta 32 por IPv4 /32 y hasta 128 por IPv6 /128.

Eso ya se midió en listas de tamaño real. 100k IPv4 /32 ocupan 71 MiB (748 B/prefijo). Cerca de 1M IPv4 /32 ocupan 560 MiB (588 B/prefijo). 100k IPv6 /128 ocupan 472 MiB (4,952 B/prefijo). El árbol es un grafo de millones de punteros que el GC de Go marca en cada ciclo, y esa marca sube CPU de GC y pausas assist de todo el proceso Traefik, no solo de este almacén. Con GOGC=100 el objetivo de heap duplica el heap vivo. `Contains` no asigna; el disparador es el tamaño de la lista guardada, no el tráfico.

Dejarlo así hace que un feed de hosts de tamaño de producción infle el proceso entero y que el GC pague el grafo en cada ciclo.

Priority: P2 — dolor real de operador, con radio limitado a quien carga listas grandes de hosts

## Implementation
Cada familia pasa a un almacén partido: mapa de hosts sin punteros más el trie radix que ya existía. Un prefijo a longitud máxima de familia (IPv4 /32, IPv6 /128; IPv4-mapped se remapea a IPv4 /32 como hasta ahora) entra en el mapa; cualquier prefijo más corto sigue en el trie. La clave IPv4 es el `uint32` big-endian de los 4 bytes; la IPv6 es `[16]byte` copiada en la pila. Las etiquetas de match se internan en una tabla por Helper (id 0 es la cadena vacía y no se libera; recuento de refs y lista libre), de modo que los mapas guardan ids y no strings. `Contains` consulta el mapa primero — un host es el prefijo más largo posible de su familia — y si no hay hit cae al trie. Las firmas públicas de Helper, el `RWMutex`, Count, el replace sin subir Count, RemoveCIDR y Reset no cambian. Yaegi v0.16.1 cargó `map[[16]byte]uint32`; no hizo falta el fallback a `struct{ hi, lo uint64 }` ni el mapa anidado.

## What this changes
**Operators.** Un proceso que carga listas grandes de hosts /32 y /128 en este almacén ocupa un orden de magnitud menos de heap y el GC marca muchos menos punteros; no hay clave nueva que fijar.
**Admin users.** None.
**Developers.** None.
**End users.** None.

## Merge readiness
In progress. 1 items remain.

Priority: P2 — dolor real de operador, con radio limitado a quien carga listas grandes de hosts
Reviewed head: ee4d1cdf
Owner decision: None.

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
| Branch | 2026-11-22-iplookup-trie-memory pushed | `git` |
| OpenSpec | iplookup-host-map-store | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/119 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
- [ ] [Path-compress the iplookup prefix trie for large non-host CIDR lists](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/knowledge/debt/2026-11-22-iplookup-path-compress-prefix-trie.md) — large lists of non-host prefixes still use one radix node per bit.


## How this fits together
Ticket 2026-11-22-iplookup-trie-memory on branch 2026-11-22-iplookup-trie-memory targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/119; CI not seen.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_standards.md) — 4 total, 0 pending, 4 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_spec.md) — 1 total, 0 pending, 1 completed
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-22-iplookup-trie-memory/devstate/2026/11/2026-11-22-iplookup-trie-memory/codereview_coverage.md) — 2 total, 0 pending, 1 completed, 1 skipped


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | ee4d1cdf2c8b482302aa7a90e0fbb74b67061f72 | Card must match the branch you measured |

### Stored data model
None.
