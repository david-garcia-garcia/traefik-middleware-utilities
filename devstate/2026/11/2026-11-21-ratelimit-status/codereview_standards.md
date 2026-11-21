# Standards

1. [judgement] Duplicated Code — `tokenbucket/status.go:52` — la regla de saturación de `durationFromNanos` está escrita tres veces (la función, y en línea en `newStatus` para `Reset` y para la espera)
   Quote:
   ```go
   if resetNanos >= maxDurationNanos {
   	reset = math.MaxInt64
   } else if resetNanos > 0 {
   	reset = time.Duration(resetNanos)
   }
   ```
   Fix: Llamar a `durationFromNanos` para `Reset` y para la espera en `newStatus`
   Status: skipped
   Argument: Judgement y trade-off documentado en el comentario de `newStatus`: cada llamada interpretada cuesta en torno a 1 µs bajo Yaegi (el plugin corre compilado y en Yaegi), y dos llamadas más por `Allow` lo pagarían en cada petición. La regla está cubierta por `TestDurationFromNanos_SaturatesAndNeverGoesNegative` y por los tests de saturación de `Reset`/`RetryAfter`.
