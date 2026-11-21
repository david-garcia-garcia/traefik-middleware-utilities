# Nitpicks

1. [hard] Symmetry and consistency — `simpleredis/resp.go:80` — en `do`, el fallo TLS de la escritura es una guarda con return propio (`resp.go:58`) y el de la lectura es un `case` dentro del `switch` de motivos `redis:unreachable` (que además discrimina por `mapped` en un caso y por `err` en los otros); mismo rol, distinta forma
   Quote:
   ```go
   switch {
   case mapped == errTLS: //nolint:errorlint // ioError returns the sentinel itself
   	sr.logger.Error("do: "+RedisTLS, "verb", commandVerb(args), "tls_error", err.Error())
   case errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF):
   	sr.logger.Warn("do: "+RedisUnreachable, "reason", "read", "verb", commandVerb(args))
   default:
   	sr.logger.Debug("do: "+RedisUnreachable, "reason", "read", "verb", commandVerb(args))
   }
   return nil, false, mapped
   ```
   Fix: En la lectura, guarda `if mapped == errTLS { sr.logger.Error(...); return nil, false, mapped }` igual que en la escritura, y dejar debajo solo la clasificación EOF / default de `redis:unreachable`
   Status: done
   Argument: En `do` la lectura usa la misma guarda `if mapped == errTLS { Error(...); return }` que la escritura; el `switch` queda solo con EOF/default de `redis:unreachable`; líneas de log compuestas en la llamada.
2. [judgement] Linear coding — `simpleredis/pool.go:286` — `dial` re-deriva en la llamada la decisión "este cliente envía AUTH" (`sr.user != "" || sr.pass != ""`) mientras `authArgs` (`pool.go:333`) decide la forma con `sr.user != ""`; la tabla User/Pass queda partida entre dos unidades. Juicio: son dos cláusulas y la pertenencia está clara, el impuesto de lectura es bajo
   Quote:
   ```go
   if sr.user != "" || sr.pass != "" {
   	if _, _, err = sr.do(ctx, conn, sr.authArgs()); err != nil {
   ```
   Fix: Predicado con nombre junto a `authArgs` (p. ej. `sr.sendsAuth()`), con la regla "usuario ACL o contraseña" documentada allí; `dial` pregunta `if sr.sendsAuth()`
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; dos cláusulas con pertenencia clara, el propio hallazgo lo califica de bajo coste de lectura.
