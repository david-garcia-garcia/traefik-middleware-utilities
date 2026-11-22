# Explore

## Concepts

Unidades que toca el cambio:

- `tokenbucket/clock.go` `validateClock` y `clockConfig.ttl`: la puerta sigue mirando el `ttl` del llamador; el suelo se guarda después en el mismo campo.
- `tokenbucket/memory.go` `NewMemory` y `Allow`: la caducidad es `now.Add(clock.ttl)`; una clave caducada nace con `tokens = burst`.
- `tokenbucket/redis.go` `NewRedis` y `tokenbucket/lua.go`: `EXPIRE` usa `ttlSeconds()` de ese mismo campo; un hash vacío también nace con `tokens = burst`.

```
caller ttl --validateClock--> floor = ceil(burst/rate + maxDelay seconds)
                                      |
                                      v
                              clockConfig.ttl = max(caller, floor)
                               /                          \
                     Memory expireAt              Redis EXPIRE seconds
                     idle past ttl -> full         empty hash -> full
```

Llamadas: búsqueda `NewMemory(` y `NewRedis(` en `**/*.go` del worktree. Las dos definiciones están en `tokenbucket/memory.go` y `tokenbucket/redis.go`. El resto de coincidencias está en tests y benchmarks de `tokenbucket/`. Ningún otro paquete construye un limiter.

Reproducido: `TestExplore13957_MemoryIdle` (archivo temporal, borrado) con `rate=0.5`, `burst=30`, `maxDelay=0`, `ttl=3s`. Primera ráfaga 30. Tras una pausa de 5 s en el reloj de test, otra ráfaga de 30. Un bucket correcto admitiría 2 (`0.5 * 5`).

Hechos de fuera: `knowledge/research/ext_traefik_ratelimiter_token-bucket/` (el `ttl` de Traefik viaja en ARGV y el script hace `EXPIRE`). El defecto concreto está en el issue https://github.com/traefik/traefik/issues/13957 y en la sección A2 del análisis citado en el requisito. No hay PR upstream.

## Decisions

- El suelo se aplica después de `validateClock` y se guarda en `clockConfig.ttl`. Memory y Redis siguen leyendo ese único valor. Rechazar el `ttl` corto queda descartado: lo dice el diseño y rompería a quien pase la fórmula de Traefik.
- Tope del suelo: el mayor `time.Duration` en segundos enteros, `(math.MaxInt64 / int64(time.Second)) * time.Second` (9223372036 s). No multiplicar el `ceil` en float por `time.Second`.
- `TestTTLTruncation_RejectsFractionalTTL` es el único assert que exige el ARGV de segundos igual al `ttl` del llamador (`"2"`). Con `rate=1`, `burst=2`, `maxDelay=1µs` el suelo es 3 s. El assert pasa a `"3"`. La parte que rechaza 1500 ms no se toca.
- Los dos repros se portan a `tokenbucket/traefik_issue13957_test.go` y `tokenbucket/traefik_issue13957_e2e_test.go`. Helpers con sufijo `13957`. `Allow` devuelve `Status`: admitir es `Status.Allowed`. Memoria: el caso `traefik_ttl` admite 2. Vivo: como mucho 3 (`int(0.5*4)+1`). Se conserva el control con TTL de rellenado.
- Contrato vivo: `openspec/specs/std_go_tokenbucket_allow/spec.md` dice que las entradas caducan tras el `ttl` del llamador. Propose lo cambia al TTL efectivo. `std_go_tokenbucket_lua-eval` mantiene que Memory y Redis duran lo mismo y que un `ttl` fraccionario se rechaza. `std_go_tokenbucket_status` no cambia de forma.
- `knowledge/devdocs/std_go_tokenbucket.md` hoy dice que el llamador pasa el `ttl` de Traefik y que una clave caducada nace llena. Implement actualiza esa frase para decir que `ttl` es un mínimo y que la librería lo sube al tiempo de rellenado. No se renombra el paquete.

## Open questions

- Q: ¿Qué segundos debe ver Redis cuando el llamador pasa 2 s, rate 1, burst 2 y maxDelay 1 µs?
  Rank: bounded asked — un assert en `tokenbucket/ttl_truncation_test.go` fija el ARGV al valor del llamador; el diseño pide `effectiveTTL = max(callerTTL, ceil(burst/rate + maxDelay.Seconds()))`. Búsqueda `**/*.go`: ese es el único compare del ARGV de ttl.
  Decision: resolved — el ARGV es `"3"`. El rechazo de 1500 ms sigue igual.
  By: explore

- Q: ¿El contrato público «caduca tras el ttl del argumento» cambia para importadores que este repo no lista?
  Rank: structural asked — `openspec/specs/std_go_tokenbucket_allow/spec.md` nombra la caducidad del `ttl` del llamador; la búsqueda `**/*.go` solo encuentra llamadas dentro de `tokenbucket/`, y los importadores de fuera no están en el árbol. El diseño del requisito pide subir el TTL.
  Decision: resolved — se sube el TTL efectivo. Propose pliega el delta en `std_go_tokenbucket_allow` y `std_go_tokenbucket_lua-eval` (cambio `tokenbucket-ttl-refill-floor`). La firma de `NewMemory` y `NewRedis` no cambia. `validateClock` no rechaza un ttl corto válido.
  By: propose

- Q: ¿Hay que alargar el reloj de `TestYaegi_MemoryCapAndExpiry`, que construye con rate 1e-6, burst 1 y ttl 1 s y avanza 1 s?
  Rank: additive asked — el diseño dice que subir el TTL no cambia las admisiones, solo cuánto vive una clave ociosa. El suelo aquí es 1000000 s.
  Decision: resolved — `go test -short -count=1 ./tokenbucket/` pasó con el suelo aplicado, incluida `TestYaegi_MemoryCapAndExpiry`. La sonda no se reescribe.
  By: implement

- Q: ¿Se actualiza `knowledge/devdocs/std_go_tokenbucket.md` si el requisito solo nombra el GoDoc de los constructores?
  Rank: additive incidental — el requisito pide el GoDoc de `NewMemory` y `NewRedis`; no nombra el paquete de uso.
  Decision: assumed — implement corrige la frase del `ttl` en ese paquete en el mismo cambio, para que no describa un mínimo que el código ya no cumple. No es un renombre.
  By: explore
