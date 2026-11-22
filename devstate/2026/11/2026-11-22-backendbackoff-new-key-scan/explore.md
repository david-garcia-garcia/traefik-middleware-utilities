# Explore

## Concepts

El `Gate` es un mapa in-process `keys map[string]*memEntry` bajo un solo `g.mu`. `Allow` y `Report` cogen ese mutex para toda la llamada. El coste que crece con el tamaño no está en el camino caliente de una clave ya residente: está en **admitir una clave que no está en el mapa**.

```
Allow ── g.mu ──► loadEntry(key)
                    present + expireAt futuro ──► misma entrada
                    present + TTL vencido     ──► delete; trata missing
                    missing ──► dropExpired: range de TODO g.keys
                                si len >= 65536 ──► dropOne
                                                     dropExpired otra vez
                                                     delete del primer key del range
                                insert CLOSED, crédito B
Report ── g.mu ──► lookup g.keys[key]; si existe, refresca expireAt; no recorre el mapa
Close  ── g.mu ──► keys = nil
```

### Unidades

| Nombre | Path | Trabajo |
|---|---|---|
| `Gate` | `backendbackoff/gate.go` | Un mutex, el mapa, `maxMemorySources = 65536`, `memEntry` sin enlaces de lista (`expireAt` sí; `key`/`prev`/`next` no). |
| `Allow` / `loadEntry` | `backendbackoff/allow.go` | Admisión + creación de clave. Cada clave nueva paga `dropExpired` (y `dropOne` en el tope) con `g.mu` cogido. |
| `dropExpired` / `dropOne` | `backendbackoff/allow.go` | Recorrido completo del mapa. `dropOne` vuelve a llamar `dropExpired` y borra un slot arbitrario (orden de iteración del mapa). |
| `Report` | `backendbackoff/allow.go` | Outcome de un intento admitido. No crea claves. Sí escribe `expireAt = now+TTL` sobre una entrada presente, incluso si el TTL de Allow ya venció (`TestReport_AfterTTLStillRecords`). |
| `tokenbucket.Memory` | `tokenbucket/memory.go` | Forma de referencia (#105): lista de expiración con punteros concretos (`makeRoomLocked`, `moveToBackLocked`, `head`/`tail`). El paquete `backendbackoff` no puede importarlo. |
| Contrato vivo | `openspec/specs/std_go_backendbackoff_allow/spec.md` | TTL perezoso en un Allow posterior, refresh en cada Allow (también denies), tope 65536, caducadas primero, luego un slot arbitrario. |
| Uso | `knowledge/devdocs/std_go_backendbackoff.md` | Gate in-memory, sin ticker, no importar `tokenbucket`. No nombra el coste del barrido en clave nueva. |
| Deuda | `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md` | Misma laguna: el gate se quedó con el patrón que #105 quitó de `tokenbucket.Memory`. |

### Call sites que importan

Raíces buscadas: `backendbackoff/*.go`, `**/*.go` del módulo (import `backendbackoff.`), `openspec/specs/`, `knowledge/devdocs/index.md` + `std_go_backendbackoff.md` + `std_go_tokenbucket.md`, `knowledge/research/index.md`, `knowledge/debt/`.

| Qué | Conteo | Dónde |
|---|---:|---|
| Defs de `loadEntry` / `dropExpired` / `dropOne` | 1 / 1 / 1 | `allow.go` |
| Callers de `loadEntry` | 1 | `Allow` |
| Callers de `dropExpired` | 2 | `loadEntry`, `dropOne` |
| Callers de `dropOne` | 1 | `loadEntry` |
| Literal `memEntry{` de producción | 1 | `allow.go` insert de clave nueva |
| Literales `memEntry{` en tests del paquete | 0 | `backendbackoff/*_test.go` |
| Lecturas de `gate.keys[...]` en tests | 6 | `zzz_gate_test.go` (crédito / estado; no construyen la entrada) |
| Import del paquete desde otro paquete Go del módulo | 0 de producción | Solo GOPATH Yaegi en `zzz_gate_yaegi_test.go` (y la copia temporal de repro) |
| API pública | `New`, `Allow`, `Report`, `Close`, `SetNowForTest` | Firmas fuera de alcance de este ticket |

Un cambio de campos no exportados en `memEntry` / `Gate` se migra aquí: no hay constructores a mano en tests y no hay callers internos fuera de `backendbackoff/`.

### Reproduce

**reproduced** — tests copiados solo para medir, ejecutados desde el worktree, y borrados después.

- `go test -count=1 -v -run '^TestScale_GateNewKeyCostGrowsWithMapSize$' ./backendbackoff` — PASS 47.53 s.

  | Residentes | Este run | Tabla del dump |
  |---:|---|---|
  | 0 | 1.036 µs | 1 µs |
  | 4.096 | 30.461 µs | 32 µs |
  | 16.384 | 126.761 µs | 145 µs |
  | 65.000 | 721.48 µs | 1.3 ms |
  | 65.536 (tope) | 1.802507 ms | 2.1 ms |

- `go test -count=1 -v -run '^TestScale_YaegiGateNewKeyCost$' ./backendbackoff` — PASS 41.71 s.

  | Residentes | Este run | Tabla del dump |
  |---:|---|---|
  | 0 | 36.438 µs | 36 µs |
  | 4.096 | 1.020203 ms | 1.1 ms |
  | 16.384 | 5.078845 ms | 4.7 ms |

El coste por clave nueva crece con `len(g.keys)` en compilado y en Yaegi. En el tope compilado hay un segundo recorrido (`dropOne`). El fill hasta 65536 vía `Allow` es O(n²); el número absoluto varía con la máquina, la pendiente no.

### Hechos de fuera

In-tree. `go.mod` pin `github.com/traefik/yaegi v0.16.1`. Yaegi: `knowledge/research/ext_traefik_plugins_yaegi-select/`, `ext_traefik_plugins_useunsafe/`, `ext_traefik_plugins_yaegi-unsafe/`, `ext_traefik_plugins_yaegi-generics/`. `container/list` y `unsafe` ya están vetados en `requirement.md` Constraints y en el comentario de `tokenbucket/memory.go` (Value `any`). No se escribió research nuevo.

## Decisions

La costura **no está elegida**. Explore lista opciones; propose (tech lead) elige.

### Opciones (ninguna tomada)

1. **Lista de expiración con punteros concretos**, copiada de `tokenbucket/memory.go` (`key`/`prev`/`next` en `memEntry`, `head`/`tail` en `Gate`, `makeRoomLocked` O(1) por clave nueva, `moveToBackLocked` al refrescar `expireAt`). Evidencia a favor: #105 ya midió y sustituyó el mismo patrón; nota de deuda; Yaegi-safe (sin `container/list`, sin `unsafe`, sin `select`). Diferencia respecto a tokenbucket: este paquete tiene `Report`, que también escribe `expireAt`.
2. **Otra estructura O(1) amortizado Yaegi-safe** (heap mínimo por `expireAt`, anillo, dos mapas live/expired). No está vetada. Evidencia en contra relativa: más allocs y más llamadas de método bajo Yaegi (`moveToBackLocked` en tokenbucket se inlinó por eso: `tokenbucket/memory.go` comentario sobre `BenchmarkYaegiMemoryAllow/rotating-10k`).
3. **Barrido amortizado** (K entradas por clave nueva, no todo el mapa). No cumple Desired «O(1) amortized»: en el tope el peor caso sigue siendo O(n) y el mutex sigue serializando.
4. **Shards de mapa**. Reparte el lock; el barrido por shard sigue siendo O(n/shards), no O(1). No resuelve solo el Desired.

### Alternativas rechazadas (evidencia las descarta)

- `container/list` — Constraints; `tokenbucket/memory.go` «its any-typed Value is a Yaegi risk».
- `unsafe` — Constraints; spec Yaegi `useunsafe` false; loader dual-gate `ext_traefik_plugins_useunsafe`.
- `select` + canales por llamada en el fuente del paquete — Constraints; `ext_traefik_plugins_yaegi-select` (slice de cases compartido entre goroutines).
- Importar `tokenbucket` — spec `std_go_backendbackoff_allow` «MUST NOT import `tokenbucket`»; Out of scope.
- Preferir CLOSED (o no-OPEN) al expulsar en el tope — Out of scope; el dump Desired y el spec dicen «one arbitrary slot». La «Dirección de arreglo» del scale-hunt § 3 no manda.

### Contrato vivo

`std_go_backendbackoff_allow` — requisito *Memory map bounds idle keys*: TTL perezoso en un Allow posterior; Allow refresca el idle deadline en cada llamada, incluidas las denegadas; tope 65536; caducadas primero, luego un slot arbitrario. El paquete no importa `tokenbucket`.

El hermano `std_go_tokenbucket_allow` deja «which live source is removed» sin especificar mientras el código de Memory hace pop LRU. Ese precedente no elige la costura de este ticket.

## Open questions

- Q: ¿Qué costura hace que admitir una clave nueva sea O(1) amortizado en compilado y Yaegi, con el mismo comportamiento observable de `std_go_backendbackoff_allow`?
  Rank: bounded asked — `loadEntry`/`dropExpired`/`dropOne` y `memEntry` viven solo en `backendbackoff` (1 def cada uno, 1 literal de producción, 0 constructores en tests; raíces `backendbackoff/*.go` y `**/*.go` del módulo); Desired nombra «O(1) amortized» y el mismo comportamiento del spec.
  Decision: resolved — by propose, pointing to design.md (opción 1, lista de expiración con punteros concretos).
  By: propose

- Q: Si propose elige una lista ordenada por `expireAt`, ¿`Report` también reenlaza cuando refresca `expireAt`?
  Rank: bounded incidental — 1 cuerpo de producción (`allow.go` `Report`); 6 lecturas de `gate.keys` en `zzz_gate_test.go` (incluye `TestReport_AfterTTLStillRecords`); el spec nombra refresh en Allow, no en Report; raíces `backendbackoff/*.go`.
  Decision: resolved — by propose, pointing to design.md Decision 2 (Report sigue escribiendo `expireAt` y reenlaza).
  By: propose

- Q: Tras una lista, ¿«one arbitrary slot» sigue sin especificar qué clave viva se quita (como `std_go_tokenbucket_allow`), aunque el código haga pop de la cabeza LRU?
  Rank: additive asked — el spec vivo ya dice slot arbitrario; Unknowns nombra este punto; no cambia callers de Allow/Report.
  Decision: resolved — by propose, pointing to design.md (la cabeza es un slot arbitrario válido; el spec no se aprieta).
  By: propose

- Q: ¿Este cambio cierra `knowledge/debt/2026-11-21-backendbackoff-gate-new-key-scan.md`?
  Rank: additive asked — Unknowns lo nombra; la nota es el mismo barrido.
  Decision: resolved — by propose, pointing to design.md y tasks.md 3.1 (borrar la nota al aterrizar el apply).
  By: propose

- Q: ¿Qué nombres de benchmark compilado y Yaegi muestran los números del Desired?
  Rank: additive asked — Desired «Benchmarks show compiled and Yaegi numbers»; Unknowns pide los nombres exactos.
  Decision: resolved — by propose, pointing to design.md Decision 5 (`BenchmarkAllow/new-key-full-table` y `BenchmarkYaegiAllow/new-key-full-table`).
  By: propose

- Q: ¿Hace falta un test de tope/expulsión a 65536 (caducadas primero, `len` se queda en 65536) que hoy no existe en `backendbackoff/`?
  Rank: additive asked — Current «Cap/eviction unit test at 65536 … not found»; Desired mismo comportamiento observable.
  Decision: resolved — by propose, pointing to design.md Decision 5 (casos a–d, `storeEntryForTestExpiryList`).
  By: propose

- Q: Si la costura reenlaza en cada Allow de una clave ya guardada, ¿el techo de `TestAllocAllowWarm` (1 alloc / 64 B) sigue siendo el contrato del camino caliente?
  Rank: bounded incidental — 1 bench y 1 test de techo en `zzz_bench_test.go`; Desired pide O(1) en clave nueva, no un rewrite del camino caliente; raíces `backendbackoff/*.go`.
  Decision: resolved — by propose, pointing to design.md Goals (`TestAllocAllowWarm` se mantiene).
  By: propose

- Q: El paquete de uso `knowledge/devdocs/std_go_backendbackoff.md` no menciona el coste del barrido en clave nueva ni una lista de expiración. ¿Se documenta en este cambio?
  Rank: additive incidental — el paquete existe; el hueco es de uso, no de Language (Gate/Allow/Report/Credit ya están); esta fase no escribe usage (produce atendido espera al humano y el run para en explore).
  Decision: assumed — aparcado para `opd-devdocsimpact` / produce atendido; no se escribe Language ni usage ahora. No hay término nuevo hasta que propose nombre la costura.
  By: explore
