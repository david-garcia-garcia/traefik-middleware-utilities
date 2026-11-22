# Issues

- [x] note large  dest `tokenbucket/memory.go` `makeRoomLocked` (lista de caducidad) y bancos `tokenbucket/bench_test.go` / `tokenbucket/bench_yaegi_test.go`
  Why: el ticket pidió diferir `Memory.dropExpired` / `dropOne` hasta tener bancos compilado y Yaegi; dest ya no tiene esas funciones y ya tiene esos bancos.
  Taken: dest sustituyó `dropExpired`/`dropOne` por `makeRoomLocked` en `2026-11-21-ratelimit-benchmarks-key-scaling`; `BenchmarkMemoryAllow` y `BenchmarkYaegiMemoryAllow` cubren clave nueva / tabla llena. No hay fichero de deuda nuevo (la superficie pedida ya está en dest).
