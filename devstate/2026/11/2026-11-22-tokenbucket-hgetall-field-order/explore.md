# Explore

## Concepts

- `allowScript` (`tokenbucket/lua.go`) es el script EVAL del bucket Redis. Hoy hace `HGETALL` y toma `last`/`tokens` por posición. Su único llamador de producto es `Redis.Allow` en `tokenbucket/redis.go` (1).
- `Memory.Allow` (`tokenbucket/memory.go`) no pasa por Redis.
- El contrato vivo es `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`: exige `#rl_source == 4` y la siembra de hash vacío (`tokens = burst`, `last = now`).
- El digest `allowScriptDigest` se calcula de `allowScript` en init. No hay un SHA escrito a mano.
- Copia aparte, no llamadora: `simpleredis/bench_test.go` `tokenBucketScript` repite el `HGETALL` posicional. Raíces buscadas: `tokenbucket/`, `simpleredis/*_test.go` y `*.go` con `hgetall` / `#rl_source`, y `openspec/specs/std_go_tokenbucket_lua-eval/`. Llamadores de `allowScript`: 1 en producción, 3 tests que inspeccionan el texto (`fake_redis_test.go`, `repro_epoch_idle_burst_test.go`, `repro_hunt_clock_last_backward_test.go`). Ninguno exige el literal `#rl_source == 4`.

```
Allow
  |
  +-- Memory.Allow -------- memEntry (no hash)
  |
  +-- Redis.Allow
        |
        Eval(allowScript)
          HMGET last, tokens   (hoy: HGETALL por índice)
          ambos truthy? --no--> seed tokens=burst, last=t
                 |
                refill, consume, HSET, EXPIRE
```

- Reproducción: `TestTraefik14297_HGetAllFieldOrder` en el worktree de análisis, contra `127.0.0.1:16379` y `127.0.0.1:16380`, con el script sin arreglar. Control `last` primero: 5 de 10 en Redis y Dragonfly. `tokens` primero en listpack: 10 de 10 en los dos (fallo). Hashtable: Redis falla (tokens primero con 0 rellenos, 10 de 10); Dragonfly hace SKIP (ningún tamaño puso `tokens` primero).
- Hechos de fuera: `knowledge/research/ext_traefik_ratelimiter_token-bucket/` (script Traefik con `HGETALL`); `knowledge/research/ext_redis_hmget_missing-field/` y `knowledge/research/ext_dragonfly_hmget_missing-field/` (campo ausente = booleano `false` en ambos motores). Issue https://github.com/traefik/traefik/issues/14297 y PR https://github.com/traefik/traefik/pull/14300.

## Decisions

- Leer `last` y `tokens` con `HMGET` por nombre y tratar el bucket como existente solo si ambos valores son truthy; si no, conservar la siembra `tokens = burst`, `last = t`. El resto del script no cambia.
- Rechazado: seguir con `HGETALL` y ordenar los campos en Lua. El orden no es un contrato.
- Rechazado: forzar listpack con `CONFIG SET`. Redis no lo permite dentro del script y cambiaría el servidor compartido.
- Contrato vivo: modificar `std_go_tokenbucket_lua-eval` (la presencia deja de ser `#rl_source == 4`). No hace falta un spec nuevo. La siembra, el persist-max de `last`, el acuerdo memoria/Redis y los tests vivos siguen.
- No tocar `tokenbucket/memory.go` ni el digest a mano.
- El paquete de uso `knowledge/devdocs/std_go_tokenbucket.md` sigue bastando para llamar al limitador. No se escribe Language nuevo.

## Open questions

- Q: ¿Dragonfly (Lua 5.4) devuelve `false` en `HMGET` cuando el campo no está, igual que Redis?
  Rank: additive asked — la comprobación no cambia llamadores; el diseño vinculante pide verificarlo en el test vivo
  Decision: resolved — EVAL en 127.0.0.1:16379 y 127.0.0.1:16380 devolvió `boolean false` para `last` y `tokens` en una clave ausente. `rl_source[1] and rl_source[2]` cae en la siembra.
  By: explore
- Q: ¿El spec vivo que exige `#rl_source == 4` se reescribe a la lectura por nombre?
  Rank: bounded asked — el contrato `std_go_tokenbucket_lua-eval` ya tiene llamadores y esta corrida los enumeró (1 Eval en `tokenbucket/redis.go`; raíces `tokenbucket/` y ese spec); Desired nombra el `HMGET`
  Decision: resolved — propose modifica ese requisito. La siembra de hash vacío y el persist-max de `last` se quedan.
  By: explore
- Q: ¿La copia `tokenBucketScript` de `simpleredis/bench_test.go` recibe el mismo `HMGET`?
  Rank: additive incidental — es un fixture de benchmark, no un llamador de `allowScript`; ningún criterio del requisito lo nombra
  Decision: assumed — se deja como está en esta corrida. Queda anotado en `knowledge/debt/2026-11-22-simpleredis-bench-tokenbucket-hgetall.md`.
  By: explore
