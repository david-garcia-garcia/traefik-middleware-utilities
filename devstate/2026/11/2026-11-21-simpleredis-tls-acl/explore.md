# Explore

## Concepts

### Unidades que toca el cambio

| Unidad | Ruta | Trabajo hoy |
|---|---|---|
| `Config` | `simpleredis/config.go` | Ajustes congelados en `New`: `Host`, `Pass`, `Database`, pool, timeouts, reintentos, `Logger`. No tiene `User` ni TLS. |
| `New` | `simpleredis/simpleredis.go` | Copia `Config` campo a campo en `SimpleRedis` (`host`, `pass`, `database`, …). No marca. |
| Sentinelas | `simpleredis/simpleredis.go` | `ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrUnsupportedReply`, `ErrPoolWait` (envuelve `ErrUnreachable`). |
| `dial` | `simpleredis/pool.go` | `net.Dialer{Timeout: DialTimeout}.DialContext(ctx, "tcp", host)`; cualquier fallo de marcado es `redis:unreachable` (reintentable). Luego `AUTH <pass>` (un argumento) y `SELECT`; fallo ahí → `handshakeFailed=true` (no se reintenta). |
| `borrowSocket` | `simpleredis/pool.go` | Único llamador de `dial` (1 sitio, `pool.go:184`). |
| `do` / `ioOrContext` / `ioError` | `simpleredis/resp.go` | `SetDeadline` con el resto del presupuesto; error de E/S → `ioError`: `os.ErrDeadlineExceeded` → `redis:timeout`, resto → `redis:unreachable`. `ioError` tiene 1 llamador (`ioOrContext`). |
| `watchConnClose` | `simpleredis/resp.go` | `context.AfterFunc(ctx, conn.Close)` para desbloquear lecturas. |
| `exec` / `shouldRetry` | `simpleredis/commands_exec.go` | Reintenta `redis:unreachable` por identidad (`err == errUnreachable`) y respuestas tipo LOADING; no reintenta `handshakeFailed`, timeout, noauth. |
| Contrato vivo | `openspec/specs/std_go_simpleredis_tcp-session/spec.md` | `First command dials TCP`: "MUST NOT use TLS"; `AUTH and SELECT run once per dial` (forma de un argumento implícita); `New records settings` (congelar en `New`; `DialTimeout` = un marcado TCP). |
| Contrato de logs | `openspec/specs/std_go_simpleredis_slog-events/spec.md` | `<site>: <cause>`; nunca `Pass`, claves ni valores. |
| Devdoc | `knowledge/devdocs/std_go_simpleredis.md` | `_Avoid_` incluye TLS (línea 8). |
| Infra e2e | `docker-compose.yml`, `e2e/simpleredisprobe/plugin.go`, `Test-Integration.ps1` | Redis y Dragonfly en claro, sin usuario ACL. |

### Cómo encajan

```
Config{Host, User?, Pass, Database, TLS?}
   │ New: copia; TLS → Clone() (congela)
   ▼
SimpleRedis ──exec──► borrowSocket ──► dial
                                        │ TLS == nil : net.Dialer.DialContext        (hoy)
                                        │ TLS != nil : tls.Dialer{NetDialer, Config}.DialContext
                                        │              (DialTimeout cubre TCP + handshake;
                                        │               ServerName vacío → host de la dirección)
                                        │   error de verificación / alerta / no-TLS → centinela TLS (no reintento)
                                        │   resto → redis:unreachable (reintento, como hoy)
                                        ▼
                                AUTH [user] pass  (si User o Pass)  → SELECT
                                        ▼
                                 do ── ioOrContext ── ioError
                                        alerta remota en lectura (TLS 1.3 cert de cliente) → centinela TLS
```

### Llamadas relevantes (contadas)

- `sr.dial(`: 1 sitio (`simpleredis/pool.go:184`). Raíz: worktree completo, `*.go`.
- `ioError(`: 1 llamador (`ioOrContext`, `simpleredis/resp.go`). Raíz: `simpleredis/`.
- Literales `Config{…}` / `simpleredis.Config{…}`: 197 en todo el worktree (`*.go`), de ellos 2 fuera de tests (`e2e/simpleredisprobe/plugin.go:79`, `:85`). `tokenbucket` y `windowcounter` reciben `*SimpleRedis`, no construyen `Config`. Añadir campos no rompe ninguno (valor cero = comportamiento actual).
- `DialTimeout:` fijado en 9 sitios, todos en tests (`simpleredis/*_test.go`, `windowcounter/repro_lock_during_get_test.go`); su semántica en TCP plano no cambia.
- Comparaciones de `RedisUnreachable` / `IsUnreachable` / `ErrNoAuth` fuera de `simpleredis/` en código no-test: 0.

### Reproducción

La "falla" es la capacidad ausente. Reproducido en el worktree (base `30b807f` + commits de prepare):

- `go doc ./simpleredis Config` → solo `Host`, `Pass`, `Database` (sin `User` ni campo TLS).
- `crypto/tls` no aparece en ninguna fuente no-test de `simpleredis/`; `pool.go:260` marca `DialContext(ctx, "tcp", sr.host)`.
- Línea base: `go test -short -count=1 ./simpleredis/` → `ok` (17,2 s).

Mediciones nuevas (sonda desechable en el scratchpad, Yaegi v0.16.1 solo `stdlib.Symbols`, Go 1.25.6; contenedores `redis:7-alpine` 7.4.10 y Dragonfly v1.40.2 creados y borrados en esta ejecución):

| # | Medida | Resultado |
|---|---|---|
| M1 | `tls.Config.Clone()` interpretado | Funciona: puntero distinto; mutar el original después no afecta al clon; `Clone()` de nil → nil; un callback interpretado sobrevive al clon. |
| M2 | Callbacks interpretados (`VerifyPeerCertificate`, `GetClientCertificate`) × 16 goroutines | 0 fallos, 16 llamadas cada uno; un callback que devuelve error falla el marcado con su texto. |
| M3 | Servidor solo TLS 1.2 | Ida y vuelta correcta, negocia TLS 1.2. |
| M4 | `ServerName` vacío | Se infiere del host de la dirección (`localhost:port` verifica SAN `localhost`; `127.0.0.1` sin SAN IP → `x509.HostnameError`). |
| M5 | `AfterFunc` cierra un `tls.Conn` con lectura bloqueada | La lectura vuelve a los 100 ms (`use of closed network connection`). |
| M6 | Coste de handshake (50 marcados, loopback, TLS 1.3 P-256) | Interpretado 0,75 ms vs TCP 0,24 ms; compilado 0,72 ms vs 0,26 ms. |
| M7 | Cliente TLS contra Redis real en claro | **No** falla rápido: espera hasta `DialTimeout` (200 ms) → `context deadline exceeded`. Contra el fake sí falla en ~0 ms (`RecordHeaderError`). |
| M7b/c | Cliente en claro contra puerto TLS | Redis: `EOF` en ~1 ms. Dragonfly: respuesta `-ERR Bad TLS header…`. |
| M8/M10 | Sin certificado de cliente, servidor lo exige (Redis por defecto; Dragonfly con CA) | TLS 1.3: primera lectura `remote error: tls: certificate required`. TLS 1.2: marcado `remote error: tls: handshake failure`. |
| M9/M10 | `AUTH user pass` por TLS | Redis y Dragonfly: correcto → `+OK`; contraseña mala, usuario inexistente o `""` → `WRONGPASS…`; usuario `nopass` + `""` → `+OK` (Redis). |
| M11 | Clase de error de una alerta remota bajo Yaegi | `errors.As(*net.OpError)` → true, `Op == "remote error"`; `errors.As(tls.AlertError)` → **false**. En loopback Windows, TLS 1.3 a veces da RST (`Op "read"`) en lugar de la alerta. |
| — | Arranque de Dragonfly | `--tls` sin `--requirepass` ni `--tls_ca_cert_file` no arranca: `TLS configured but no authentication method is used!`. |

### Hechos externos usados

- `ext_traefik_plugins_yaegi-tls` (nuevo) — Yaegi + `crypto/tls`.
- `ext_go_crypto-tls_client-dial` (nuevo) — `tls.Dialer`, inferencia de `ServerName`, alertas como `net.OpError`, `Close`.
- `ext_redis_tls` (nuevo), `ext_dragonfly_tls` (nuevo).
- `ext_redis_auth`, `ext_dragonfly_auth` (actualizados: AUTH de dos argumentos).
- `ext_go-redis_options_tls-config`, `ext_go-redis_auth_username` (nuevos).
- In-tree: `ext_go-redis_yaegi-compatibility` (por qué existe simpleredis), devdoc `std_go_simpleredis`.

## Decisions

- Superficie TLS: `Config.TLS *tls.Config`; nil = TCP plano (comportamiento actual intacto). Misma forma que go-redis `Options.TLSConfig`.
- Congelar: `New` guarda `cfg.TLS.Clone()` (medido bajo Yaegi, M1). La congelación es superficial (`RootCAs`, `Certificates`, callbacks compartidos); se documenta.
- Marcado: `dial` usa `tls.Dialer{NetDialer: &net.Dialer{Timeout: DialTimeout}, Config: clon}.DialContext(ctx, "tcp", host)` cuando hay TLS. No `tls.DialWithDialer` (ignora ctx). No se deriva `ServerName` de `Host`: lo hace `crypto/tls`.
- `DialTimeout` pasa a acotar TCP + handshake TLS de un intento; sigue sin haber tercer mando.
- Usuario ACL: `Config.User`; AUTH se envía cuando `User` o `Pass` no están vacíos; con `User` → `AUTH <user> <pass>`, sin `User` → `AUTH <pass>` (como hoy).
- Errores TLS deterministas → un centinela exportado nuevo, no reintentado, que no envuelve `ErrUnreachable`; se clasifica en `dial` (verificación, alerta remota, registro no-TLS) y en `ioError` (alerta remota en lectura). Todo lo demás sigue siendo `redis:unreachable` reintentable.
- Material de certificados e2e: generado al preparar la prueba (stdlib, PKCS#8), no versionado.
- Rechazado: superficie estrecha (`CAFile`/`CertPEM`/`ServerName`/`Insecure` como campos propios) — duplica `tls.Config` como abstracción paralela (`Consume before produce`) y no cubre versiones, cifrados ni raíces del sistema sin crecer.
- Rechazado: rechazar o borrar callbacks de `tls.Config` en `New` — medido que funcionan (M2); quitarlos sería una variación sin dueño ni petición.
- Rechazado: centinela solo en el handshake — no cubre el rechazo TLS 1.3 que aflora en la primera lectura (Tensions; M8/M11).
- Rechazado: `errors.As(tls.AlertError)` para detectar alertas — medido false (M11); la forma es `*net.OpError{Op: "remote error"}`.
- Rechazado: ignorar `User` sin `Pass` como go-redis — autenticaría en silencio como `default`.
- Contrato vivo: `std_go_simpleredis_tcp-session` (MODIFIED `First command dials TCP`, `AUTH and SELECT run once per dial`, `New records settings and does not dial`; ADDED para el centinela TLS sin reintento). `std_go_simpleredis_slog-events` solo si el centinela añade una línea de log nueva (`dial: <texto>` a Error). `std_go_simpleredis_live-e2e` para la prueba en vivo TLS/ACL.

## Open questions

- Q: ¿Qué superficie expone TLS: `Config.TLS *tls.Config` o campos propios más estrechos?
  Rank: additive asked — campo nuevo en `Config`; los 197 literales existentes siguen compilando con valor cero; requirement.md "Alcance acordado / Dentro 2: TLS: poder conectar a Redis/Dragonfly por TLS (verificación de servidor con CA propia o raíces del sistema, nombre de servidor, y certificado de cliente para mTLS)"
  Decision: assumed — `Config.TLS *tls.Config`, nil = TCP plano. Cubre CA propia (`RootCAs`), raíces del sistema (`RootCAs` nil), `ServerName` y mTLS (`Certificates`) sin abstracción paralela; mismo dueño de forma que go-redis (`ext_go-redis_options_tls-config`).
  By: explore

- Q: ¿Qué se hace con los callbacks interpretados (`VerifyPeerCertificate`, `GetClientCertificate`) que un `*tls.Config` público permite asignar?
  Rank: additive incidental — no añade unidad ni rama: el campo los transporta por ser el tipo estándar; ninguna línea Dentro los nombra y prepare listó "exponer en la API callbacks interpretados" como fuera de alcance inferido
  Decision: assumed — ni API propia, ni rechazo, ni borrado en `New`. Se documenta que el tipo los admite, que bajo Yaegi v0.16.1 se midió que funcionan (M2, 16 handshakes concurrentes) y que la librería no los prueba ni los promete en spec.
  By: explore

- Q: ¿`tls.Config.Clone()` funciona bajo Yaegi para congelar `Config.TLS` en `New`?
  Rank: additive asked — copia de un campo nuevo dentro de `New`, que ya congela `Config`; Dentro 2 (TLS) más el requisito vivo "After New, writes to the caller's Config … MUST NOT change … the copied knobs"
  Decision: resolved — sí (M1: puntero distinto; mutar `RootCAs`/`ServerName`/callback del original tras el clon no afecta; nil → nil). `New` guarda el clon. La copia es superficial (doc oficial "shallow clone"): mutar el `*x509.CertPool` compartido sí se vería; se documenta, no se copia en profundidad.
  By: explore

- Q: ¿Quién es dueño del nombre de servidor que se verifica (identidad Host) cuando `tls.Config.ServerName` está vacío?
  Rank: additive asked — no se crea lógica propia; Dentro 2 nombra "nombre de servidor"
  Decision: resolved — el dueño es `crypto/tls` (`dial` clona y fija `ServerName` = host de la dirección; `golang/go@go1.25.6:src/crypto/tls/tls.go`; medido M4 y en Redis real M8e). simpleredis pasa `Config.Host` como dirección y no reconstruye `ServerName`; si el llamante lo fija, gana el suyo.
  By: explore

- Q: ¿Quién es dueño del usuario ACL con el que se autentica la sesión (identidad user)?
  Rank: additive asked — campo nuevo; requirement.md "Dentro 1: Usuario ACL: poder autenticar con usuario y contraseña (`AUTH <user> <pass>`)"
  Decision: assumed — ningún host ni vecino lo calcula: Traefik no aporta identidad hacia Redis y `tokenbucket`/`windowcounter` reciben un `*SimpleRedis` ya construido. Dueño = el llamante vía `Config.User`, congelado en `New` como `Pass`.
  By: explore

- Q: ¿Qué hace `User` sin `Pass`: error en `New`, se ignora, o `AUTH user ""`?
  Rank: additive asked — rama nueva en `dial` sobre un campo que crea este cambio; Dentro 1
  Decision: assumed — AUTH se envía si `User` o `Pass` no están vacíos; con `User` siempre forma de dos argumentos (`AUTH user ""` si `Pass` vacío). Medido: usuario `nopass` acepta `""` (`+OK`); usuario con contraseña → `WRONGPASS` → `redis:noauth` (ya no reintentado). Sin error nuevo en `New`. Se aparta a propósito de go-redis v9.22.0, que ignora `Username` sin `Password` (`ext_go-redis_auth_username`) y dejaría la sesión como `default` en silencio.
  By: explore

- Q: ¿Qué fallos TLS van a un centinela nuevo no reintentable y cuáles siguen como `redis:unreachable`?
  Rank: additive incidental — centinela exportado nuevo y ramas nuevas en `dial` (1 llamador) e `ioError` (1 llamador) que solo se alcanzan con TLS configurado; ninguna línea Dentro nombra el centinela: viene de la "Propuesta de diseño (orientativa)": "Un centinela de error para fallos de verificación de certificado, no reintentable"
  Decision: assumed — un único centinela exportado `ErrTLS` (`errors.New(RedisTLS)`, `RedisTLS = "redis:tls"`), sin envolver `ErrUnreachable` (es un fallo de configuración que el operador debe corregir, como `ErrNoAuth`), no reintentado, devuelto tal cual (sin envolver la causa: `Error()` exacto `redis:tls`); el texto de `crypto/tls` va en el atributo `tls_error` de la línea de log a Error del sitio que lo vio (`dial: redis:tls` / `do: redis:tls`; `dial` añade Warn con `verb` si falló AUTH/SELECT). Sin helper `IsTLS` (como `ErrNoAuth`). Va al centinela: `*tls.CertificateVerificationError` (CA o nombre), `*net.OpError` con `Op == "remote error"` (alerta del servidor: cert de cliente rechazado, versión) y `tls.RecordHeaderError` (peer no-TLS que responde). Sigue en `redis:unreachable` reintentable: fallo TCP, `DialTimeout`/ctx durante el handshake, y cualquier otro error de marcado (p. ej. error de un `VerifyPeerCertificate` del llamante). Detección con `errors.As` a tipos concretos (medido bajo Yaegi, M11 y sonda del mantenedor); no es la aserción a la interfaz `net.Error` que prohíbe el spec. `errors.As(tls.AlertError)` no sirve (M11). Spec: `std_go_simpleredis_tls-handshake`, `std_go_simpleredis_slog-events`.
  By: propose

- Q: ¿Cómo se trata el rechazo de certificado de cliente en TLS 1.3, que aflora en la primera lectura y hoy se reintenta sin `Pass`?
  Rank: additive incidental — rama en `ioError` (1 llamador, `ioOrContext`) inalcanzable sin TLS; lo nombra Tensions, no una línea Dentro
  Decision: assumed — `ioError` clasifica `*net.OpError{Op: "remote error"}` como el centinela TLS, así el caso con `Pass` (lectura de AUTH) y sin `Pass` (primer comando) dan el mismo error y ninguno se reintenta. Medido en Redis 7.4.10 y Dragonfly v1.40.2 (M8, M10). En loopback Windows un RST puede adelantarse a la alerta (M11): ese caso cae en `redis:unreachable` y se reintenta una vez; se documenta, no se intenta distinguir.
  By: explore

- Q: ¿Qué pasa con un cliente TLS contra un Redis real en claro?
  Rank: additive asked — no añade rama; Dentro 2
  Decision: resolved — medido (M7): Redis no responde al ClientHello, el marcado espera `DialTimeout` y devuelve `context deadline exceeded` → `redis:unreachable` reintentable (2 × 200 ms con valores por defecto, dentro de `CommandTimeout` 900 ms). No se distingue de un peer lento; se documenta en el devdoc. `RecordHeaderError` (fallo rápido) solo aparece si el peer escribe primero.
  By: explore

- Q: ¿Funciona contra un servidor solo TLS 1.2 y hay que fijar `MinVersion`?
  Rank: additive asked — sin código propio; Dentro 2
  Decision: resolved — funciona (M3 fake, M8d Redis forzando 1.2). No se fija `MinVersion`: el valor por defecto de Go para clientes es TLS 1.2 (pkg.go.dev); el llamante puede cambiarlo en su `tls.Config`.
  By: explore

- Q: ¿Qué acota `DialTimeout` cuando hay TLS?
  Rank: bounded asked — cambia el texto de un contrato existente ("`DialTimeout` SHALL cap one TCP dial attempt (`net.Dialer.Timeout`)"); llamadas que lo fijan: 9, todas tests (raíz: worktree `*.go`, patrón `DialTimeout:`), y en TCP plano su efecto no cambia, así que no hay que migrar ninguna; Dentro 2
  Decision: assumed — `DialTimeout` acota un intento completo: conexión TCP + handshake TLS (semántica de `tls.Dialer`, `ext_go_crypto-tls_client-dial`); el ctx del comando sigue mandando por encima. No se añade un mando de handshake (el spec prohíbe un mando que no acote algo propio y el handshake loopback cuesta ~0,5 ms extra, M6).
  By: explore

- Q: ¿Cuánto cuesta el handshake por marcado y hace falta reanudación de sesión?
  Rank: additive asked — sin código propio; Dentro 2
  Decision: resolved — ~0,72 ms compilado / ~0,75 ms interpretado frente a ~0,25 ms TCP en loopback (M6). Se paga una vez por socket; el pool lo amortiza. No se añade `ClientSessionCache` (el llamante puede ponerlo en su `tls.Config`).
  By: explore

- Q: ¿Se puede registrar `User` en logs?
  Rank: additive incidental — ninguna línea lo pide; el requirement solo prohíbe `Pass` y el texto de respuesta de AUTH
  Decision: assumed — no se añade `User` a ninguna línea (Bound the ask); las líneas de fallo de AUTH siguen como hoy (`dial: redis:noauth`, `verb AUTH`). `Pass` y la respuesta del peer siguen prohibidos.
  By: explore

- Q: ¿Cómo se genera y versiona el material de certificados de e2e?
  Rank: additive incidental — infraestructura de prueba nueva; los tests aparecen en la "Propuesta de diseño (orientativa)", no en una línea Dentro
  Decision: assumed — generado al preparar la prueba con stdlib (`crypto/x509`, claves PKCS#8 porque Dragonfly rechaza SEC1; SAN `localhost`, `redis-tls`, `dragonfly-tls`, `127.0.0.1`), no versionado (sin claves privadas ni caducidad en el repo). Un único generador, en código de test (`simpleredis/tls_fixture_test.go`): lo usan el fake TLS en proceso y `TestWriteTLSFixtureFiles`, que escribe los PEM en `SIMPLEREDIS_TLS_FIXTURE_OUT` (se salta sin esa variable). Composición: solo los jobs Go E2E de CI (`e2e-redis`, `e2e-dragonfly`) generan el material en `$RUNNER_TEMP`, arrancan con `docker run` un Redis TLS (`:6383`) y un Dragonfly TLS (`:6384`) con certificado de cliente obligatorio (Dragonfly arranca gracias a `--tls_ca_cert_file`), crean el usuario ACL `app` con `ACL SETUSER` y exportan `SIMPLEREDIS_LIVE_{REDIS,DRAGONFLY}_TLS` + `SIMPLEREDIS_LIVE_TLS_DIR`. No se toca `docker-compose.yml`, `e2e/simpleredisprobe` ni `Test-Integration.ps1`: Pester no es la prueba de verbos de SimpleRedis (`knowledge/devdocs/std_go_test-suites.md` `_Avoid_`) y la carga real en Traefik ya la midió el mantenedor (sonda C).
  By: propose

- Q: ¿Cómo se reconcilia el contrato vivo que dice "MUST NOT use TLS" y el devdoc que lista TLS en `_Avoid_`?
  Rank: bounded asked — modifica requisitos vivos de `std_go_simpleredis_tcp-session` con llamantes: literales de `Config` enumerados (197 en `*.go`, 2 fuera de tests en `e2e/simpleredisprobe/plugin.go`), todos siguen válidos con valor cero; requirement.md Tensions "Propose debe modificar ese requisito, no añadir uno paralelo" y Dentro 2
  Decision: assumed — propose hace MODIFIED en `std_go_simpleredis_tcp-session` de `First command dials TCP` (TLS cuando `Config.TLS` no es nil; Unix sockets siguen prohibidos), de `AUTH and SELECT run once per dial` (forma con usuario) y de `New records settings and does not dial` (copia `User`, clon de `TLS`, `DialTimeout` = TCP + handshake). El centinela TLS no reintentado no va como ADDED en `tcp-session` sino en una hoja nueva `std_go_simpleredis_tls-handshake` (FindSpecHost: `tcp-session` ya lleva tres requisitos modificados, el umbral de fold; la clasificación TLS es una capacidad nueva). Además: `std_go_simpleredis_slog-events` fold (ADDED línea TLS a Error; MODIFIED `A peer reply is not published` para que `redis:tls` sea centinela propio) y `std_go_simpleredis_live-e2e` fold (ADDED TLS + ACL en vivo, compilado y Yaegi). El devdoc `std_go_simpleredis.md` se actualiza al implementar (quitar TLS de `_Avoid_`, añadir uso).
  By: propose

- Q: ¿Cerrar un `tls.Conn` (cancelación vía `watchConnClose` o `release` de un socket sucio) puede bloquear?
  Rank: additive asked — sin cambio en esas unidades; Dentro 2
  Decision: resolved — la cancelación desbloquea la lectura al momento (M5). `tls.Conn.Close` envía `close_notify` con un plazo de escritura de 5 s si no hay `Write` en curso (`golang/go@go1.25.6:src/crypto/tls/conn.go`); en la goroutine de `AfterFunc` no frena al comando; en `release` (diferido en `runOnConn`, dentro del camino del comando) solo bloquearía si el buffer de envío del kernel estuviera lleno (inferencia sobre el código de Go, no medido). Se acepta y se documenta; no se cambia `pooledConn.close`.
  By: explore
