# simpleredis: TLS y usuario ACL

Origen: petición de un usuario de la librería, trasladada en chat por el mantenedor el 2026-11-21. Alcance confirmado por el mantenedor en chat ("me parece un buen alcance").

## Queja del usuario (texto recibido, sin editar en contenido)

Lo que le falta a simpleredis según el usuario:

| Falta | Afecta | ¿Importa aquí? |
|---|---|---|
| Pipelining (varios comandos en una escritura) | Rendimiento | Poco. Cada petición envía un comando. Solo ayudaría a un flush por lotes, y un EVAL sobre varias claves ya hace ese trabajo. |
| TLS (marca TCP plano, `net.Dialer` en `pool.go`) | Seguridad | Sí, si Redis no está en una red privada |
| Usuario ACL (AUTH solo con contraseña) | Seguridad | Sí, si tu Redis usa usuarios ACL |
| Varios endpoints / Cluster / Sentinel (un único Host) | Alta disponibilidad | Sí, si necesitas Redis en HA |

## Alcance acordado

Dentro:

1. **Usuario ACL**: poder autenticar con usuario y contraseña (`AUTH <user> <pass>`), no solo con contraseña (`AUTH <pass>`).
2. **TLS**: poder conectar a Redis/Dragonfly por TLS (verificación de servidor con CA propia o raíces del sistema, nombre de servidor, y certificado de cliente para mTLS).

Fuera (decidido por el mantenedor):

- **Pipelining**: no se hace. Cada verbo envía un comando; `MSetEX` y `Eval` ya cubren escrituras por lotes.
- **Varios endpoints / Sentinel / Cluster (HA)**: aplazado hasta conocer la topología concreta del usuario que lo pide.

## Propuesta de diseño presentada al mantenedor (orientativa; explore/propose la validan o la ajustan)

- `Config.User` (ACL) y `Config.TLS *tls.Config` (nil = TCP plano), clonado en `New` para congelarlo como el resto de `Config`.
- `dial` usa `tls.Dialer` cuando hay TLS, y `AUTH user pass` cuando hay `User`.
- Un centinela de error para fallos de verificación de certificado, no reintentable.
- Tests: fake TLS, test Yaegi, e2e Redis y Dragonfly con TLS y usuario ACL.
- Actualizar `knowledge/devdocs/std_go_simpleredis.md` (hoy lista TLS en `_Avoid_`, línea 8) y escribir una nota de investigación en `knowledge/research/ext_traefik_plugins_yaegi-tls/` con las mediciones.

## Investigación de viabilidad ya hecha (2026-11-21, sondas desechables, fuera del repo)

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

## Restricciones del proyecto relevantes

- simpleredis existe porque go-redis no corre bajo Yaegi (`knowledge/research/ext_go-redis_yaegi-compatibility/`); todo debe seguir siendo stdlib e interpretable.
- Nunca registrar `Pass` (ni el texto de respuesta del AUTH, que puede citarlo).
