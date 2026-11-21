## Motivation
Un middleware de Traefik que use `simpleredis` para hablar con Redis o Dragonfly solo puede conectar por TCP en claro y solo puede autenticarse con `AUTH <pass>`. Un usuario de la librería pidió las dos cosas que le faltan para un despliegue real, y el mantenedor confirmó el alcance: TLS y usuario ACL. El pipelining y los endpoints múltiples (Sentinel, Cluster) quedan fuera.

TLS: `dial` abría siempre `net.Dialer.DialContext(ctx, "tcp", host)`, y `Config` no tenía dónde poner una CA, un nombre de servidor ni un certificado de cliente. El contrato vivo lo prohibía de forma expresa (`std_go_simpleredis_tcp-session`, "MUST NOT use TLS") y el devdoc lo listaba en `_Avoid_`. Un Redis fuera de una red privada, o un servicio gestionado que solo acepta TLS o exige mTLS, no era alcanzable, salvo exponiendo `Pass`, las claves y los valores en claro por la red.

Usuario ACL: `Config` solo tenía `Pass`, y `dial` enviaba la forma de un argumento, `AUTH <pass>`, que Redis interpreta como el usuario `default`. Un Redis o Dragonfly que da a la aplicación un usuario propio (por ejemplo `app`) no podía autenticar a ese usuario. La única salida era abrir el usuario `default` con su contraseña.

Si no se resuelve, la librería no sirve para los despliegues que exigen cifrado en tránsito o usuarios ACL. Quien la usa tiene que mantener Redis en una red privada sin TLS o trabajar con el usuario `default`.

Priority: P2 — un usuario de la librería no puede conectar con despliegues de Redis/Dragonfly con TLS o usuarios ACL, y hoy solo tiene un rodeo: red privada en claro o usuario `default`.

## Implementation
`Config` gana dos campos aditivos. `User string` cambia AUTH a la forma de dos argumentos `AUTH <user> <pass>`, que se envía también con `Pass` vacío (un usuario ACL `nopass`). Sin `User`, se mantiene `AUTH <pass>`. `TLS *tls.Config` vale nil por defecto, y nil es TCP en claro. `New` guarda `cfg.TLS.Clone()` para congelarlo como el resto de `Config`. Es una copia superficial: el pool de `RootCAs`, los elementos de `Certificates` y los callbacks siguen siendo compartidos, y así queda documentado en el campo. La librería no añade campos paralelos (`CAFile`, `ServerName`…). CA propia, raíces del sistema, `ServerName`, mTLS y versiones se configuran en el `tls.Config` del llamante. Si `ServerName` está vacío, `crypto/tls` lo toma del host de `Host`.

El marcado pasa por un único punto (`openNetConn`). Sin TLS, usa el `net.Dialer` de antes. Con TLS, usa `tls.Dialer{NetDialer: &net.Dialer{Timeout: DialTimeout}, Config: clon}.DialContext`, que respeta el `ctx` del comando durante el handshake. `DialTimeout` pasa a acotar la conexión TCP y el handshake TLS juntos. No hay un timeout nuevo. AUTH y SELECT corren después del handshake, igual en los dos caminos.

Los fallos TLS deterministas devuelven un centinela nuevo exportado: `ErrTLS`, con texto exacto `redis:tls` (`RedisTLS`). No envuelve `ErrUnreachable` y no se reintenta: `shouldRetry` solo reintenta `errUnreachable` por identidad, así que no hubo que tocarlo. Un único clasificador, `isTLSRejection`, usa `errors.As` contra tipos concretos: `*tls.CertificateVerificationError`, `tls.RecordHeaderError` y `*net.OpError` con `Op == "remote error"`. No hace aserción a `net.Error` ni usa `tls.AlertError`, que bajo Yaegi no reconoce la alerta. El clasificador se llama en dos sitios:

- en `dial`, para fallos del handshake;
- en `ioError`, para la alerta que TLS 1.3 envía en la primera lectura cuando el servidor rechaza la falta de certificado de cliente. Puede llegar en la respuesta de AUTH o en el primer comando.

Con eso, los dos casos dan el mismo `redis:tls`. Un fallo TCP, un handshake que agota `DialTimeout` o cualquier otro error de marcado siguen siendo `redis:unreachable` y se reintentan. El texto de `crypto/tls` no va en el error devuelto. Va en el atributo `tls_error` de una línea a nivel Error (`dial: redis:tls` o `do: redis:tls`). `User` y `Pass` no se registran.

Cobertura, según el diff:

- **Fake TLS en proceso, compilado:**
  - ida y vuelta con CA propia y `ServerName` vacío;
  - mTLS;
  - servidor solo TLS 1.2;
  - `TLS` nil sigue en claro;
  - `Config.TLS` congelado en `New`;
  - el handshake cuenta contra `DialTimeout`;
  - cancelación durante el handshake;
  - rechazos que dan `redis:tls` en una sola conexión: CA desconocida, nombre que no coincide, IP sin SAN, certificado de cliente rechazado en el handshake TLS 1.2 y en la primera lectura TLS 1.3, con y sin AUTH, y peer en claro que responde al ClientHello;
  - peer silencioso y TCP rechazado, que siguen en `redis:unreachable`;
  - formas de AUTH y `WRONGPASS` → `redis:noauth`;
  - líneas de log.
- **Yaegi v0.16.1 (solo stdlib, contra el fake):** mTLS con usuario ACL, y una CA desconocida que da `errors.Is(err, ErrTLS)`.
- **En vivo:** en los jobs de CI `e2e-redis` y `e2e-dragonfly` (y en implement, en local contra los mismos contenedores en Docker), contra `redis:7-alpine` y Dragonfly v1.40.2. Ambos tienen TLS con certificado de cliente obligatorio y usuario ACL `app`. Se prueba lo siguiente:
  - compilado: ida y vuelta mTLS + ACL, contraseña ACL errónea → `redis:noauth`, sin certificado de cliente → `redis:tls`, CA no confiable → `redis:tls`;
  - interpretado: ida y vuelta.

Los certificados se generan en cada ejecución con stdlib y no se versionan. Ningún test del diff arranca Traefik. Tampoco se prueban las raíces del sistema ni los callbacks de `tls.Config`. CI medida en implement sobre el commit `88407858` (run 44886646789): 8/8 checks en verde, incluidos `Go E2E Redis` y `Go E2E Dragonfly`; los logs de los jobs no se han leído, así que que los tests TLS en vivo corrieran (y no se saltaran) se infiere de la configuración del job, no se ha visto. La CI tras las correcciones de code review se mide en pullrequest.

## What this changes
**Operators.** Cuando un middleware active TLS, un rechazo TLS aparece como línea Error `dial: redis:tls` o `do: redis:tls` con el detalle en `tls_error`, y no se reintenta. Un cliente TLS apuntado a un puerto en claro de Redis no falla rápido: da `redis:unreachable` tras `DialTimeout`.
**Admin users.** None.
**Developers.** `simpleredis.Config` añade `User` (AUTH de dos argumentos, también con `Pass` vacío) y `TLS *tls.Config` (nil = en claro, clonado de forma superficial en `New`), y hay un centinela nuevo `ErrTLS`/`RedisTLS` (`redis:tls`): no se reintenta y `IsUnreachable` es false. `DialTimeout` acota ahora TCP + handshake, y los tests en vivo TLS se activan con `SIMPLEREDIS_LIVE_{REDIS,DRAGONFLY}_TLS` y `SIMPLEREDIS_LIVE_TLS_DIR`; los literales de `Config` existentes siguen compilando y se comportan igual.
**End users.** None.

## Merge readiness
Ready for review. 0 items remain.

Priority: P2 — un usuario de la librería no puede conectar con despliegues de Redis/Dragonfly con TLS o usuarios ACL, y hoy solo tiene un rodeo: red privada en claro o usuario `default`.
Reviewed head: 63d8d632
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888513108 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-21-simpleredis-tls-acl pushed | `git` |
| OpenSpec | simpleredis-tls-acl | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/103 | pr-host |
| CI | build 44888513108 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888513108 | https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888513108 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [std_go_simpleredis_live-e2e](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/changes/archive/2026-11-21-simpleredis-tls-acl/proposal.md) — modified
- [std_go_simpleredis_slog-events](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/changes/archive/2026-11-21-simpleredis-tls-acl/proposal.md) — modified
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/changes/archive/2026-11-21-simpleredis-tls-acl/proposal.md) — modified
- [std_go_simpleredis_tls-handshake](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/changes/archive/2026-11-21-simpleredis-tls-acl/proposal.md) — added

Completed:
- [std_go_simpleredis_live-e2e](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/specs/std_go_simpleredis_live-e2e/spec.md) — modified
- [std_go_simpleredis_slog-events](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/specs/std_go_simpleredis_slog-events/spec.md) — modified
- [std_go_simpleredis_tcp-session](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/specs/std_go_simpleredis_tcp-session/spec.md) — modified
- [std_go_simpleredis_tls-handshake](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/openspec/specs/std_go_simpleredis_tls-handshake/spec.md) — added


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-21-simpleredis-tls-acl on branch 2026-11-21-simpleredis-tls-acl targeting master; PR https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/103; CI build 44888513108 succeeded https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/44888513108.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| ¿Qué superficie expone TLS: `Config.TLS *tls.Config` o campos propios más estrechos? | additive asked — campo nuevo en `Config`; los 197 literales existentes siguen compilando con valor cero; requirement.md "Alcance acordado / Dentro 2: TLS: poder conectar a Redis/Dragonfly por TLS (verificación de servidor con CA propia o raíces del sistema, nombre de servidor, y certificado de cliente para mTLS)" | assumed — `Config.TLS *tls.Config`, nil = TCP plano. Cubre CA propia (`RootCAs`), raíces del sistema (`RootCAs` nil), `ServerName` y mTLS (`Certificates`) sin abstracción paralela; mismo dueño de forma que go-redis (`ext_go-redis_options_tls-config`). | explore |


## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_standards.md) — 7 total, 0 pending, 4 completed, 3 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_nitpicks.md) — 2 total, 0 pending, 1 completed, 1 skipped
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_spec.md) — 4 total, 0 pending, 3 completed, 1 skipped
[Scope](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-11-21-simpleredis-tls-acl/devstate/2026/11/2026-11-21-simpleredis-tls-acl/codereview_coverage.md) — 3 total, 0 pending, 0 completed, 3 skipped


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 6 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 63d8d632ff7d440b13a637fee9b3244ebc1eb2b1 | Card must match the branch you measured |

### Stored data model
None.
