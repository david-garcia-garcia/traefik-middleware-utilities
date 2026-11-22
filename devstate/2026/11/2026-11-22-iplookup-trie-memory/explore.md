# Explore

## Concepts

El paquete `iplookup` es un almacén CIDR en proceso. Quien llama pasa un `net.IP` que ya eligió. Helper no reconstruye la dirección del cliente (spec `std_go_iplookup_family-match`, requisito «Caller owns the looked-up address»; Language **Helper** en el packet de uso).

Unidades que este cambio tocaría:

| Nombre | Path | Trabajo |
|---|---|---|
| Helper | `iplookup/helper.go` | Almacén CIDR público: `New`, `AddCIDR`, `AddEntry`, `HostCIDR`, `RemoveCIDR`, `Contains`, `Reset`, `Count`. Guarda `mu sync.RWMutex`, `ipv4Tree`, `ipv6Tree`, `count`. |
| ipRadixTree | `iplookup/tree.go` | Trie por bit, no exportado, una familia. Superficie candidata. |
| radixNode | `iplookup/tree.go:6-12` | Un bit del recorrido CIDR: `isEndpoint`, `prefixLen`, `metadata string`, `left`/`right *radixNode`. |
| familyWalk / prefixWalk / bitAt | `iplookup/tree.go:128-158` | Pasan la dirección o el CIDR a la cadena de bits; remap IPv4-mapped `ones-96` cuando la máscara tiene 128 bits. |
| Specs vivas | `openspec/specs/std_go_iplookup_family-match/spec.md`, `std_go_iplookup_cidr-store/spec.md`, `std_go_iplookup_match-label/spec.md` | Aislamiento de familia, alta/baja/reset/count/concurrencia, etiqueta de longest-prefix. |
| Packet de uso | `knowledge/devdocs/std_go_iplookup.md` | Cómo un autor de middleware llama a Helper. |

```
plugin / test
    |
    v
 Helper  (API pública — se mantiene)
    |  RWMutex, count
    +-- ipv4Tree *ipRadixTree ----+
    +-- ipv6Tree *ipRadixTree ----+--> internos de tree.go (candidato)
                                      un radixNode por bit de prefijo
                                      familyWalk / prefixWalk / bitAt
```

Call sites (raíces: `*.go` y `*.md` del worktree por `ipRadixTree`, `newIPRadixTree`, `radixNode`, `iplookup.New`, import `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`; `iplookup/*.go` por `h := New()`):

- `ipRadixTree` / `radixNode` / `newIPRadixTree`: **10** usos de producción en `iplookup/helper.go` (2 campos, 4× `newIPRadixTree` en `New`/`Reset`, `treeFor`, `insert`, `contains`, `remove`). Tipos y métodos en `iplookup/tree.go`. **0** tests nombran esos identificadores. **0** otros paquetes.
- API pública Helper, importadores de producción in-tree: **0** (solo el paquete `iplookup`). Tests in-package: **20** `New()` (`zzz_helper_test.go` 16, `zzz_tree_test.go` 1, `zzz_entry_test.go` 3). Fuente del probe Yaegi en `zzz_helper_yaegi_test.go`: **3** `iplookup.New()`. Un plugin Traefik fuera del árbol importaría Helper, no `ipRadixTree`.

Reproduce: **path run**, pass. Copia temporal del test de caza en `iplookup/zzz_scale_hunt_iplookup_trie_memory_repro_test.go`, comando `go test -count=1 -timeout 15m -v -run TestScale_TreeMemoryPerPrefix ./iplookup/` desde el worktree, luego borrado. `go version go1.25.6 windows/amd64`.

| Prefijos | Heap | Bytes/prefijo | Carga |
|---|---:|---:|---:|
| v6=false n=100000 | 71.3 MiB | 748 | 89 ms |
| v6=false n=999836 | 560.3 MiB | 588 | 1.072 s |
| v6=true n=100000 | 472.2 MiB | 4952 | 291 ms |

`n` es `h.Count()` tras los inserts (colisiones IPv4 /32 con seed 1: 1M → 999836). Tabla de caza en `master 1f5ec454`: 71 MiB / 747 B / 92 ms; 560 MiB / 588 B / 1.1 s; 472 MiB / 4,952 / 0.3 s.

`unsafe.Sizeof(radixNode{})` = **48** (throwaway, borrado). `unsafe.Sizeof(ipRadixTree{})` = 8.

Contains a 100k IPv6 /128, compilado, 20k reps tras la carga (throwaway, borrado): hit media **106 ns** (última dirección insertada, camino caliente); miss media **51 ns** (`2001:db8::1`). Contains en Yaegi sigue sin medir.

Hechos de fuera: informe de caza §2 `iplookup` (solo lectura); research `ext_traefik_plugins_yaegi-generics`, `ext_traefik_plugins_useunsafe`, `ext_traefik_plugins_yaegi-unsafe`, `ext_traefik_plugins_yaegi-select` (Yaegi v0.16.1, sin `unsafe` en el loader de este producto). Sin escritura nueva en `knowledge/research/`.

## Decisions

- Chosen: mantener la API pública de Helper y los tres contratos vivos `std_go_iplookup_*`; la superficie candidata son los internos no exportados de `ipRadixTree` (`iplookup/tree.go` y los dos campos árbol de Helper).
- Chosen: mantener el aislamiento de familia en Helper (`To4()` → árbol IPv4, incluido IPv4-mapped) y el remap IPv4-mapped en `prefixWalk`; el almacén detrás de cada familia es la elección abierta.
- Chosen: explore lista OPTIONS de representación; propose (tech lead) elige la forma. Nada Chosen aquí.
- Chosen: la identidad del cliente sigue en quien llama; Helper no es dueño de esa dirección (no reconstruir HTTP / X-Forwarded-For).
- Rejected: ampliar este ticket a `ReplaceAll` atómico / construir fuera del lock y swap — Out of scope; Desired mantiene `Reset` como el clear.
- Rejected: partir el `RWMutex` compartido o cambiar la granularidad del lock — Out of scope.
- Rejected: elegir Patricia o rangos empaquetados (u otra forma) en explore — Out of scope «Picking the store representation in this phase».
- Rejected: commitear el repro de caza o una suite nueva en esta fase — reproducir y borrar.
- Live contract: `std_go_iplookup_family-match`, `std_go_iplookup_cidr-store`, `std_go_iplookup_match-label`.

## Open questions

- Q: Qué representación segura para Yaegi puede bajar memoria y punteros de GC ≥10× en listas grandes de hosts aleatorios (IPv4 /32, IPv6 /128) sin romper los contratos vivos?
  Rank: structural asked — reescritura del almacén radix existente (`ipRadixTree` / un `radixNode` por bit); Desired nombra bajar memoria y punteros de GC al menos un orden de magnitud manteniendo cada contrato actual
  Decision: resolved — by propose, pointing to design.md (Decision 1: host maps + existing trie).
  By: propose

- Q: Cuánto cuestan la carga y Contains en Yaegi de una representación nueva?
  Rank: additive asked — medición de una representación que este cambio introduciría; Desired nombra medir compilado y Yaegi (tiempo de carga y Contains)
  Decision: resolved — host maps + trie, IPv6 key `[16]byte` (Yaegi v0.16.1 lo aceptó). 100k hosts, `go test -bench` en esta máquina. Compilado load IPv4 13.0 ms (dest 89–92 ms), IPv6 25.2 ms (dest 291 ms). Compilado Contains IPv4 hit 14.07 ns / miss 20.96 ns; IPv6 hit 31.97 ns / miss 29.86 ns (dest IPv6 hit 106 ns / miss 51 ns). Yaegi load IPv4 1.36 s, IPv6 1.48 s. Yaegi Contains IPv4 hit 7769 ns / miss 14936 ns; IPv6 hit 6654 ns / miss 9602 ns. Memoria HeapInuse 100k: IPv4 12.6 B/prefix (dest HeapAlloc 748), IPv6 28.2 B/prefix (dest 4,952).
  By: implement

- Q: Cuál es la latencia de Contains a 100k IPv6 /128?
  Rank: additive asked — no cambia un contrato existente; Desired nombra que la latencia de lookup no debe regresar de forma material; Unknowns del requirement la daban por no medida
  Decision: resolved (compiled) — 20k reps tras la carga en esta máquina: hit media 106 ns (último /128 insertado, camino caliente), miss media 51 ns (`2001:db8::1`). Caveat: el último camino está caliente por el insert; no es una mezcla de tráfico Traefik. Yaegi sigue sin medir (fila anterior).
  By: explore

- Q: Cuál es el radio de explosión de sustituir los internos de `ipRadixTree` frente a mantener la API de Helper?
  Rank: bounded asked — callers in-tree enumerados (10 usos de `ipRadixTree` en `helper.go`, 0 otros paquetes, 0 tests que nombren el tipo no exportado; 0 importadores de producción de Helper); Desired mantiene Helper y las tres specs `std_go_iplookup_*`
  Decision: resolved — sustituir solo internos no exportados; mantener las firmas de Helper y las tres specs vivas. Los plugins fuera del árbol importan Helper, no `ipRadixTree`. `familyWalk` / `prefixWalk` siguen siendo la costura de familia salvo que propose muestre que el almacén nuevo absorbe ese remap sin cambiar Helper.
  By: explore
