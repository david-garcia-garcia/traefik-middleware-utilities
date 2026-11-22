# Alinear el script de bench de simpleredis con HMGET por nombre

IssueKey: 2026-11-22-tokenbucket-hgetall-field-order
Size: large
Action: note

## Why this follow-up

`simpleredis/bench_test.go` guarda una copia de `tokenBucketScript` que sigue leyendo `HGETALL` por posición (`#rl_source == 4`). El limitador de producto es `tokenbucket/lua.go`.

## Why it was not taken

El diseño vinculante nombra solo `allowScript`. El bench no es un llamador de ese script. Unattended solo toma filas pequeñas sobre archivos que esta corrida crea.

## Risks

El benchmark de simpleredis sigue midiendo el script con el orden de campos frágil, y un lector puede tomarlo por el script de producto.
