# Requirement
IssueKey: 2026-11-21-simpleredis-tls-acl

El texto del solicitante es un brief de diseño completo (`ticket/source.md`, spec del llamante con `issueHost: local`). Se copia íntegro a continuación; las secciones posteriores (`Current (code)`, `Affected`, `Out of scope`, `Unknowns`, `Tensions`) son el análisis de prepare y no sustituyen al brief. La "Propuesta de diseño" del brief es orientativa: explore/propose la validan o la ajustan.

---

## simpleredis: TLS y usuario ACL

Origen: petición de un usuario de la librería, trasladada en chat por el mantenedor el 2026-11-21. Alcance confirmado por el mantenedor en chat ("me parece un buen alcance").

### Queja del usuario (texto recibido, sin editar en contenido)

Lo que le falta a simpleredis según el usuario:

| Falta | Afecta | ¿Importa aquí? |
|---|---|---|
| Pipelining (varios comandos en una escritura) | Rendimiento | Poco. Cada petición envía un comando. Solo ayudaría a un flush por lotes, y un EVAL sobre varias claves ya hace ese trabajo. |
| TLS (marca TCP plano, `net.Dialer` en `pool.go`) | Seguridad | Sí, si Redis no está en una red privada |
| Usuario ACL (AUTH solo con contraseña) | Seguridad | Sí, si tu Redis usa usuarios ACL |
| Varios endpoints / Cluster / Sentinel (un único Host) | Alta disponibilidad | Sí, si necesitas Redis en HA |

### Alcance acordado

Dentro:

1. **Usuario ACL**: poder autenticar con usuario y contraseña (`AUTH <user> <pass>`), no solo con contraseña (`AUTH <pass>`).
2. **TLS**: poder conectar a Redis/Dragonfly por TLS (verificación de servidor con CA propia o raíces del sistema, nombre de servidor, y certificado de cliente para mTLS).

Fuera (decidido por el mantenedor):

- **Pipelining**: no se hace. Cada verbo envía un comando; `MSetEX` y `Eval` ya cubren escrituras por lotes.
- **Varios endpoints / Sentinel / Cluster (HA)**: aplazado hasta conocer la topología concreta del usuario que lo pide.

### Propuesta de diseño presentada al mantenedor (orientativa; explore/propose la validan o la ajustan)

- `Config.User` (ACL) y `Config.TLS *tls.Config` (nil = TCP plano), clonado en `New` para congelarlo como el resto de `Config`.
- `dial` usa `tls.Dialer` cuando hay TLS, y `AUTH user pass` cuando hay `User`.
- Un centinela de error para fallos de verificación de certificado, no reintentable.
- Tests: fake TLS, test Yaegi, e2e Redis y Dragonfly con TLS y usuario ACL.
- Actualizar `knowledge/devdocs/std_go_simpleredis.md` (hoy lista TLS en `_Avoid_`, línea 8) y escribir una nota de investigación en `knowledge/research/ext_traefik_plugins_yaegi-tls/` con las mediciones.

### Investigación de viabilidad ya hecha (2026-11-21, sondas desechables, fuera del repo)

Medido con Yaegi v0.16.1, solo `stdlib.Symbols` (Traefik con `useUnsafe=false`), host Go 1.25.6:

- A — interpretado contra servidores TLS compilados en proceso: TLS 1.3, CA propia, `InsecureSkipVerify`, SAN por IP sin `ServerName`, AUTH usuario+pass, mTLS, 32 goroutines × 100 comandos, reutilización de socket ocioso tras 500 ms: todo correcto.
- B — misma sonda contra Redis 7.4.10 (`--tls-port`, `--tls-auth-clients optional`, `--requirepass`, usuario ACL `app`) y Dragonfly v1.40.2 (`--tls`, con y sin `--tls_ca_cert_file`): correcto. WRONGPASS por TLS igual que en plano.
- C — Traefik v3.7.11 real (compilado con Go 1.26.6) cargando un plugin local que marca TLS: correcto contra Redis, Redis con ACL, Dragonfly y Dragonfly mTLS; concurrencia 8 × 50 correcta.

Hallazgos:

- `tls.Dialer{NetDialer: &net.Dialer{Timeout}, Config}.DialContext` respeta `ctx` (300 ms) y `DialTimeout` (200 ms) durante el handshake; el error es `context.DeadlineExceeded`.
- `SetDeadline` sobre `tls.Conn` sigue dando `os.ErrDeadlineExceeded` (mapeo actual a `redis:timeout` intacto).
- `errors.As` alcanza `*tls.CertificateVerificationError`, `x509.UnknownAuthorityError` y `x509.HostnameError` a través del intérprete.
- TLS contra un peer en claro falla en ~1 ms con `tls: first record does not look like a TLS handshake` (medido solo contra fake, no contra Redis real).
- En TLS 1.3, el rechazo de un certificado de cliente aparece en la primera lectura (`remote error: tls: certificate required`), no en el handshake: con `Pass` cae en AUTH (fallo de handshake, no reintentado); sin `Pass` cae en el primer comando (`redis:unreachable`, reintentado).
- Dragonfly con `--tls_ca_cert_file` exige certificado de cliente; Redis lo controla con `--tls-auth-clients` (por defecto `yes`).
- Dragonfly no cargó una clave EC en formato SEC1 (`EC PRIVATE KEY`); sí en PKCS#8 (`PRIVATE KEY`).

No cubierto: `tls.Config.Clone()` bajo Yaegi; servidor solo TLS 1.2; Redis real en claro frente a cliente TLS; coste del handshake medido aparte; callbacks interpretados (`VerifyPeerCertificate`, `GetClientCertificate`) — recomendación: no exponerlos.

Fuentes de las sondas (desechables, scratchpad de la sesión): `tlsprobe-host/main.go`, `tlsprobe-host/probe/tlsprobe.go.txt`, `traefikprobe/plugin/*`, `traefikprobe/dynamic.yml`.

### Restricciones del proyecto relevantes

- simpleredis existe porque go-redis no corre bajo Yaegi (`knowledge/research/ext_go-redis_yaegi-compatibility/`); todo debe seguir siendo stdlib e interpretable.
- Nunca registrar `Pass` (ni el texto de respuesta del AUTH, que puede citarlo).

---

## Current (code)

Base: `origin/master` @ `30b807f`.

- Usuario ACL — `Config` solo tiene `Pass` ("AUTH on each new dial when non-empty"); no existe campo de usuario: `simpleredis/config.go` (`type Config`, campos `Host`/`Pass`/`Database`).
- Usuario ACL — `dial` envía `AUTH <pass>` (un argumento) cuando `sr.pass != ""`: `simpleredis/pool.go` (`func (sr *SimpleRedis) dial`). Redis interpreta la forma de un argumento como usuario `default`: `knowledge/research/ext_redis_auth/notes.md`.
- Usuario ACL — `New` copia `Pass` a `sr.pass`; no hay `user`: `simpleredis/simpleredis.go` (`func New`).
- TLS — `dial` usa `net.Dialer{Timeout: sr.DialTimeout()}.DialContext(ctx, "tcp", sr.host)`; no hay rama TLS: `simpleredis/pool.go` (`func (sr *SimpleRedis) dial`).
- TLS — `Config` no tiene campo TLS; `New` congela `Config` copiando campo a campo: `simpleredis/config.go`, `simpleredis/simpleredis.go` (`func New`).
- TLS — el contrato vivo prohíbe TLS: "The session MUST NOT dial a Unix socket and MUST NOT use TLS": `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (`Requirement: First command dials TCP`).
- TLS — el devdoc lista TLS en `_Avoid_`: `knowledge/devdocs/std_go_simpleredis.md` (línea 8).
- Centinela de verificación de certificado — no existe; sentinelas exportados actuales `ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrUnsupportedReply`, `ErrPoolWait`: `simpleredis/simpleredis.go`.
- Reintento — un fallo de dial TCP devuelve `errUnreachable` y se reintenta; un fallo de AUTH/SELECT marca `handshakeFailed` y no se reintenta: `simpleredis/pool.go` (`dial`), `simpleredis/commands_exec.go` (`shouldRetry`).
- Logging — prohibido registrar `Pass` o el texto de respuesta del peer en AUTH: `simpleredis/pool.go` (comentario en `dial`), `knowledge/devdocs/std_go_simpleredis.md` (línea 109).
- Timeout de I/O — `redis:timeout` se mapea con `errors.Is(err, os.ErrDeadlineExceeded)`, sin aserción `net.Error`: `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (`Requirement: I/O deadline is timeout, not a net.Error assert`).
- Tests fake — servidor RESP en claro sobre `net.Listener`; no hay variante TLS: `simpleredis/fake_redis_test.go` (`startFakeRedis`).
- Tests Yaegi — `simpleredis/yaegi_test.go` (p. ej. `TestYaegi_HandshakeAuthWrongPassMatchesNoAuth`); ninguno con TLS ni usuario.
- e2e — Redis (`redis:7-alpine`) y Dragonfly (`v1.40.2`) sin TLS ni usuario ACL; plugin local `simpleredisprobe`: `docker-compose.yml`, `e2e/simpleredisprobe/plugin.go`.
- Nota de investigación Yaegi + TLS — `not found` (`knowledge/research/ext_traefik_plugins_yaegi-tls/` no existe).
- Pipelining — `not found` (fuera de alcance; se cita solo para dejar constancia).
- Varios endpoints / Sentinel / Cluster — `not found`; `Config.Host` es una única dirección: `simpleredis/config.go`.

## Affected

- Paquete `simpleredis`: `simpleredis/config.go`, `simpleredis/simpleredis.go`, `simpleredis/pool.go`, `simpleredis/commands_exec.go` (clasificación de reintento), tests del paquete.
- Contrato vivo: `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (prohibición de TLS; requisito de AUTH por dial).
- Devdoc: `knowledge/devdocs/std_go_simpleredis.md`.
- Infra e2e: `docker-compose.yml`, `e2e/simpleredisprobe/plugin.go`, `Test-Integration.ps1` (material de certificados y servicios TLS/ACL no existen hoy).
- Consumidores de `simpleredis.Config` en el repo (solo lectura de impacto; el brief no les pide cambios): `tokenbucket/redis.go`, `windowcounter/limiter.go`.

## Out of scope

Decidido por el mantenedor (brief):

- Pipelining.
- Varios endpoints / Sentinel / Cluster (HA).

Inferido por prepare, no pedido (se listan, no se toman):

- Exponer en la API de simpleredis callbacks interpretados de TLS (`VerifyPeerCertificate`, `GetClientCertificate`); el brief recomienda no exponerlos.
- Cambiar `tokenbucket` o `windowcounter` para que expongan TLS/usuario en su propia configuración.
- Unix sockets (siguen en `_Avoid_` del devdoc).
- Reconfiguración de TLS o credenciales después de `New`.

## Unknowns

(Explore los resuelve o los escala; `skill:opd-research` donde dependan de un tercero.)

- `tls.Config.Clone()` bajo Yaegi: no medido (el brief lo marca como no cubierto) y la propuesta de diseño depende de él para congelar `Config.TLS` en `New`.
- Servidor solo TLS 1.2: no medido.
- Redis real en claro frente a cliente TLS: medido solo contra fake (~1 ms, `tls: first record does not look like a TLS handshake`); qué sentinel y si se reintenta queda por decidir.
- Clasificación de errores TLS: qué fallos de handshake/verificación van al centinela nuevo, cuáles a `redis:unreachable`, y cuáles no se reintentan; el rechazo de certificado de cliente en TLS 1.3 aparece en la primera lectura, no en el handshake (ver Tensions).
- `User` sin `Pass`: comportamiento no especificado (¿error en `New`, se ignora, o `AUTH user ""`?).
- Si `User` se puede registrar en logs (el brief solo prohíbe `Pass` y el texto de respuesta del AUTH).
- Coste del handshake TLS por dial (compilado e interpretado): no medido aparte.
- Cómo generar y versionar el material de certificados para e2e (CA, servidor, cliente; Dragonfly no carga claves EC SEC1, sí PKCS#8).
- Cómo interactúa `ServerName` con `Config.Host` cuando `tls.Config.ServerName` está vacío (las sondas validaron SAN por IP sin `ServerName`; hostname no medido explícitamente en el brief).

## Tensions

- Brief vs contrato vivo: el brief pide TLS; `openspec/specs/std_go_simpleredis_tcp-session/spec.md` dice "MUST NOT use TLS". Propose debe modificar ese requisito, no añadir uno paralelo.
- Brief vs devdoc: `knowledge/devdocs/std_go_simpleredis.md` línea 8 lista TLS en `_Avoid_`; el brief ya prevé actualizarlo.
- Centinela "no reintentable" vs comportamiento medido: en TLS 1.3, sin `Pass`, el rechazo de certificado de cliente aflora en el primer comando como `redis:unreachable` y hoy se reintenta (`simpleredis/commands_exec.go` `shouldRetry`); con `Pass` aflora en AUTH (no reintentado). Un centinela solo en el handshake no cubre ese caso.
- `Config.TLS *tls.Config` vs recomendación de no exponer callbacks interpretados: un `*tls.Config` público permite que el llamante asigne `VerifyPeerCertificate`/`GetClientCertificate` igualmente; el brief no dice si se rechazan, se ignoran o se documentan.
- Congelar `Config` en `New` vs `*tls.Config` (puntero mutable): el brief propone clonarlo, pero `Clone()` bajo Yaegi no está medido.
