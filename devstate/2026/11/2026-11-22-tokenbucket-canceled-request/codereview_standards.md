# Standards

1. [hard] Name for the scope — `tokenbucket/traefik_issue14173_e2e_test.go:22` — `countingProxy14173` nombra un proxy que retrasa escrituras como si contara; `holdWrites` guarda nanosegundos, no un recuento
   Quote:
   ```go
   type countingProxy14173 struct {
   	listener   net.Listener
   	holdWrites atomic.Int64 // nanoseconds each client-to-server chunk waits before it is forwarded
   }
   ```
   Fix: Renombrar el tipo a `writeHoldProxy14173` y el campo a `writeHoldNanos`
   Status: done
   Argument: Renombrados el tipo a `writeHoldProxy14173` y el campo a `writeHoldNanos` en `tokenbucket/traefik_issue14173_e2e_test.go`.
2. [hard] Bound the ask — `knowledge/opendev.json:1` — el diff de producto incluye `knowledge/opendev.json` (modelo e idiomas de OpenDev) y `.cursorignore`, ajenos a GoDoc de Allow y a los tests de ctx cancelado
   Quote:
   ```json
   {
     "model_default": "",
   ```
   Fix: Quitar ambos archivos de la rama (o llevarlos a un cambio propio si el repositorio decide versionarlos)
   Status: done
   Argument: Borrados `.cursorignore` y `knowledge/opendev.json` de la rama.
