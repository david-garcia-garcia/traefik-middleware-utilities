# Test coverage

1. [judgement] Assertion does not prove the job — `simpleredis/tls_test.go:219` — `TestTLSCallerCancelDuringHandshake` solo comprueba `errors.Is(err, context.Canceled)`; si `openNetConn` volviera a `tls.DialWithDialer` (ignora ctx; rechazado en design Decision 2), el handshake esperaría `DialTimeout` (2 s) y `dial` devolvería igualmente `context.Canceled` por el `contextStop` posterior, así que el test seguiría en verde sin probar que la cancelación corta el handshake (hoy tarda 0,05 s)
   Quote:
   ```
   pool.go:326  dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: sr.DialTimeout()}, Config: sr.tlsConfig}
   pool.go:265  if stop := contextStop(ctx); stop != nil { return nil, stop, false }
   tls_test.go  DialTimeout: 2 * time.Second ... if !errors.Is(err, context.Canceled) { t.Fatalf(...) }  // sin cota de tiempo
   ```
   Fix: Medir la duración de `Get` y exigir que termine muy por debajo de `DialTimeout` (p. ej. < 1 s) además de `context.Canceled`
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; añadir cota de tiempo al test de cancelación es una mejora de aserción opcional.
2. [judgement] Happy path only — `simpleredis/pool.go:307` — la rama nueva `dial: redis:tls` a Warn con `verb` `SELECT` (alerta TLS 1.3 leída en la respuesta a SELECT, sin `Pass` y con `Database`) no la ejecuta ningún test; `TestLogTLSFirstReadAlert` solo cubre las filas sin AUTH y con AUTH
   Quote:
   ```
   pool.go:306-307  case errors.Is(err, ErrTLS): sr.logger.Warn("dial: "+RedisTLS, "verb", verbSelect)
   test: (none)
   ```
   Fix: Añadir a `TestLogTLSFirstReadAlert` una fila con `Database` y sin `Pass` que exija `do: redis:tls` y `dial: redis:tls` a Warn, ambos con `verb` `SELECT`
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; fila adicional (SELECT sin Pass) de un test existente, no un trabajo del ticket sin probar.
3. [judgement] Happy path only — `simpleredis/resp.go:58` — la rama de escritura de `do` que registra `do: redis:tls` cuando la escritura falla con `errTLS` no tiene test; solo se prueba la rama de lectura (primera lectura TLS 1.3)
   Quote:
   ```
   resp.go:58-61  if mapped == errTLS { sr.logger.Error("do: "+RedisTLS, "verb", commandVerb(args), "tls_error", err.Error()); return nil, false, mapped }
   test: (none)
   ```
   Fix: Añadir un test que provoque un fallo TLS en la escritura y exija `redis:tls` y la línea `do: redis:tls`, u omitirlo justificando que la rama no es alcanzable en la práctica
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; la rama de escritura TLS es difícil de provocar y el propio Fix admite omitirla.
