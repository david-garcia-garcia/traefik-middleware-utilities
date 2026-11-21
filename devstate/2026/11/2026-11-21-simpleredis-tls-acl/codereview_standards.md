# Standards

1. [hard] Consume before produce — `simpleredis/tls_fixture_test.go:246` — `startSilentPeerForTest` y `startWritesFirstPlainPeerForTest` (`:275`) reimplementan el bucle listen/accept/contador/goroutine que ya posee `startAcceptFake` (`simpleredis/pool_test.go:367`, que ya devuelve el contador de accepts y ejecuta un `handle` por socket); `startStallRedis` (`commands_deadline_test.go:13`) ya es el peer silencioso
   Fix: Borrar ambos helpers y llamar a `startAcceptFake` con un `handle` que haga `drainUntilPeerCloseForTest(conn)` (silencioso) o lea el primer bloque, escriba `-ERR unknown command\r\n` y drene (writes-first); leer `atomic.LoadInt32(acceptCount)` donde hoy se llama `accepts()`
   Status: done
   Argument: Borrados `startSilentPeerForTest` y `startWritesFirstPlainPeerForTest`; `tls_test.go` usa `startAcceptFake` (handle con `drainUntilPeerCloseForTest`, o lectura del ClientHello + `-ERR unknown command` + drenaje) y `startStallRedis` en el test de cancelación; los accepts se leen con `atomic.LoadInt32(acceptCount)`.
2. [hard] Leave a trail — `simpleredis/resp.go:75` — la introducción del bloque de lectura sigue diciendo que todo otro fallo de I/O es un peer que se fue, pero el diff añadió al `switch` un brazo `errTLS` que es un rechazo TLS a nivel Error, no un peer caído
   Fix: Reescribir la línea para nombrar los tres casos: rechazo TLS (`redis:tls`, Error con `tls_error`), EOF/lectura corta (Warn) y el resto de I/O (Debug)
   Quote:
   ```go
   // A short bulk read is Warn (the peer lied about a length); every other I/O failure is a peer that went away.
   ```
   Status: done
   Argument: Reescrita la introducción del bloque de lectura en `resp.go`: nombra rechazo TLS (Error, `redis:tls`, `tls_error`), EOF/lectura corta (Warn) y el resto de I/O (Debug).
3. [hard] Leave a trail — `simpleredis/commands_exec.go:220` — el comentario de `isUnreachable` sigue afirmando que «other IO via ioError» es `redis:unreachable`, y el de `shouldRetry` (`:195`) enumera lo que no se reintenta sin nombrar `redis:tls`; tras el diff `ioError` devuelve `errTLS` y el no-reintento de TLS solo queda escrito en el comentario de un llamador (`dial`, `pool.go:255`). Fuera de los hunks, pero en la ruta que el diff reclasificó
   Fix: Corregir `isUnreachable` a «other IO via ioError except a TLS rejection (redis:tls)» y añadir `redis:tls` a la lista de no reintentados del comentario de `shouldRetry`
   Status: skipped
   Argument: outside diff; not applied unattended. Los comentarios de `isUnreachable`/`shouldRetry` en `commands_exec.go` son anteriores a `redis:tls`.
4. [hard] Bound the ask — `knowledge/opendev.json:1` — el diff de producto incluye `knowledge/opendev.json` (ajustes locales del flujo OpenDev: `model_default`, idiomas) y `.cursorignore` (`.cursorignore:1`), que genera `opendev_settings_get` y no guardan relación con TLS ni con el usuario ACL; en el checkout principal ambos están sin seguimiento
   Fix: Quitar ambos archivos de la rama (o llevarlos a un cambio propio si el repositorio decide versionarlos)
   Status: done
   Argument: Borrados `.cursorignore` y `knowledge/opendev.json` del worktree; el checkpoint del conductor registra la eliminación.
5. [hard] One job, one owner — `simpleredis/tls_fixture_test.go:153` — el archivo del generador de certificados (`tlsFixtureForTest`) también aloja una segunda unidad viva: el fake TLS (`tlsFakeOptionsForTest`, `tlsFakeStateForTest` con su mutex, `startTLSFakeRedis`, `admitTLSConnForTest`) más los peers planos; el fake RESP existente vive en su propio `fake_redis_test.go`
   Fix: Mover el fake TLS y sus tipos a `tls_fake_test.go` (o junto a `fakeRedis` en `fake_redis_test.go`) y dejar `tls_fixture_test.go` con el fixture de certificados y `TestWriteTLSFixtureFiles`; actualizar las entradas de Key files en ambos paquetes de devdocs
   Status: done
   Argument: Fake TLS (`tlsFakeOptionsForTest`, `tlsFakeStateForTest`, `startTLSFakeRedis`, `admitTLSConnForTest`, `drainUntilPeerCloseForTest`) movido a `simpleredis/tls_fake_test.go`; `tls_fixture_test.go` queda con el fixture de certificados y `TestWriteTLSFixtureFiles`; Key files actualizados en `std_go_simpleredis.md` y `std_go_test-suites.md`.
6. [judgement] Duplicated Code — `simpleredis/tls_e2e_test.go:101` — `liveTLSClientConfigForTest` repite la forma de `tlsFixtureForTest.clientTLSConfig` (`tls_fixture_test.go:143`): pool de la CA, `MinVersion: tls.VersionTLS12` y certificado de cliente opcional; solo cambia el origen de los PEM (archivos frente a memoria), así que un cambio de política TLS de los tests se debe hacer en dos sitios
   Fix: Cargar un `*tlsFixtureForTest` desde `SIMPLEREDIS_LIVE_TLS_DIR` (CA, client.crt, client.key) y llamar a su `clientTLSConfig`
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; unificar la config TLS live con el fixture en memoria es un refactor opcional de tests.
7. [judgement] Duplicated Code — `simpleredis/slog_test.go:569` — `msgAttrForTest` recorre `r.Attrs` buscando `key` en los registros de `msg` igual que `requireMsgAttr` (`slog_test.go:83`); dos recorridos casi idénticos en el mismo archivo
   Fix: Hacer que `requireMsgAttr` delegue en un único buscador de atributos (o que `msgAttrForTest` sea ese buscador y `requireMsgAttr` lo llame)
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; duplicación menor entre dos helpers de test en el mismo archivo.
