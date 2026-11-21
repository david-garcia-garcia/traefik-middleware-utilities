# Test coverage

1. [hard] Edge case untested — `windowcounter/status.go:45` — `newStatus` tiene una rama explícita para `headroom >= float64(limit)` (sin estimado, `Remaining = limit`) que evita que `int64(headroom)` desborde cuando `float64(limit)` redondea por encima de `math.MaxInt64`; ningún test la recorre y revertirla deja verde toda la suite
   Quote:
   ```go
   if headroom >= 1 && headroom >= float64(limit) {
   	remaining = limit
   } else if headroom >= 1 {
   	remaining = int64(headroom)
   }
   // test: (none)
   ```
   Fix: Peek sobre una ventana vacía con `limit` 10 y `math.MaxInt64`, en exact y buffered: `Remaining == Limit`
   Status: done
   Argument: Añadido `TestStatus_EmptyWindowHasTheWholeLimitRemaining` en `windowcounter/status_test.go` (Peek en ventana vacía, `limit` 10 y `math.MaxInt64`, exact y buffered). Verificado que falla al revertir la rama (`Remaining` -9223372036854775808) y pasa con ella.
