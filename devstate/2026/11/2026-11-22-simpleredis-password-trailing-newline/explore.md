# Explore

## Concepts

Ticket TEST + DOC: no hay cambio de AUTH. `Config.Pass` llega a AUTH tal cual; un newline de un Secret o fichero es otra contraseña. Causa en [traefik/traefik#12078](https://github.com/traefik/traefik/issues/12078).

Unidades que este cambio toca:

| Unidad | Path | Job |
|---|---|---|
| `Config.Pass` | `simpleredis/config.go` | Contraseña freeze-at-New. GoDoc hoy: «AUTH on each new dial when non-empty». No dice byte a byte ni «no trim». |
| `New` | `simpleredis/simpleredis.go` | Copia `cfg.Pass` a `sr.pass` (`pass: cfg.Pass`). Sin `Trim`. |
| `authArgs` | `simpleredis/pool.go` | `[]byte(sr.pass)` en AUTH de un argumento o `AUTH <user> <pass>`. |
| `applyDefaults` | `simpleredis/config.go` | No toca `Pass` ni `User`. |
| `replyError` | `simpleredis/resp.go` | Prefijo `WRONGPASS` → `redis:noauth` (`ErrNoAuth`). |
| Harness live AUTH | `simpleredis/simpleredis_e2e_test.go` (`runForEachLiveEngine`), `simpleredis/newtest_test.go` (`newTestRedis`) | Skip bajo `-short` o sin `SIMPLEREDIS_LIVE_*_AUTH`. El test nuevo se copia encima de esto. |
| WRONGPASS live hoy | `simpleredis/pool_e2e_test.go` (`TestLive_WrongPassword`) | `"wrong-password"` → `redis:noauth`. No cubre `"secret\n"`. |
| Formas AUTH fake | `simpleredis/pool_test.go` (`TestAuthArgumentForms`) | Filas con `Pass` `"secret"` exacto. Ninguna con newline. |
| Probe | `e2e/simpleredisprobe/plugin.go` | Único llamador de producción: `Pass: cfg.Password` sin recorte. Fuera de alcance cambiarlo. |

```
  caller Config.Pass
           │
           ▼
        New copy  (no Trim)
           │
           ▼
        sr.pass
           │
           ▼
      authArgs  []byte(sr.pass)   AUTH bulk, byte for byte
           │
           ▼
      requirepass engine
           │
     ┌─────┴──────┐
     ▼            ▼
    +OK        -WRONGPASS
  Get miss    replyError → redis:noauth
  ("secret")   ("secret\n")
```

Call sites de `Pass` en código de sesión (no tests): 4. Roots: `simpleredis/*.go` (`cfg.Pass`, `sr.pass`); `e2e/simpleredisprobe/`; `tokenbucket/` y `windowcounter/` (`simpleredis.Config` / `Pass:`). Hallados: copia en `New` (`simpleredis.go`); guarda AUTH y dos `[]byte(sr.pass)` en `authArgs` (`pool.go`); un setter de producción (`plugin.go` `Pass: cfg.Password`). `tokenbucket/` y `windowcounter/`: 0 asignaciones a `Pass`.

Reproduce: path run. Throwaway `main` (temp, `replace` al worktree, `MaxRetries: -1`) contra `SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381` y `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382`. `Get("traefik-12078-explore-probe")`:

| Motor | `Pass` | Resultado |
|---|---|---|
| Redis `:16381` | `"secret"` | `redis:miss` (AUTH OK, clave ausente) |
| Redis `:16381` | `"secret\n"` | `redis:noauth` |
| Dragonfly `:16382` | `"secret"` | `redis:miss` |
| Dragonfly `:16382` | `"secret\n"` | `redis:noauth` |

Puertos OPEN antes del probe. `TestTraefik12078_PasswordWithTrailingNewline` y `simpleredis/traefik_issue12078_e2e_test.go`: not found en este worktree (el repro de referencia vive en el otro árbol, solo lectura).

Hechos externos: `knowledge/research/ext_redis_auth/` (`WRONGPASS` → `redis:noauth`); issue [traefik/traefik#12078](https://github.com/traefik/traefik/issues/12078) (Secret k8s con newline; análisis de referencia D2). No hace falta carpeta de research nueva: AUTH ya está cubierto; el newline como otra contraseña se midió aquí.

Devdocs consumidos: `knowledge/devdocs/std_go_simpleredis.md` (How to use / Gotchas AUTH; no nombra recorte ni newline), `std_go_test-suites.md` (gate live AUTH). Language sin hueco. Produce: nada en esta fase (DOC pedida = GoDoc de `Config.Pass`).

Catálogo live AUTH: `openspec/specs/std_go_simpleredis_tcp-session` (forma `AUTH <pass>`, WRONGPASS → `redis:noauth`); `openspec/specs/std_go_simpleredis_live-e2e` (escenario Live WRONGPASS). Ninguno promete bytes literales ni cita #12078.

## Decisions

- Seam: GoDoc en `Config.Pass` (AUTH byte a byte, sin recorte; newline de Secret o fichero hace fallar AUTH; citar https://github.com/traefik/traefik/issues/12078) más copia tal cual de `TestTraefik12078_PasswordWithTrailingNewline` a `simpleredis/traefik_issue12078_e2e_test.go`. Reusar `runForEachLiveEngine` / `newTestRedis`. No tocar `New`, `authArgs` ni `applyDefaults`.
- Rechazado: recortar `Pass` (o `User`) antes de AUTH — binding y Out of scope; transform silencioso de credencial.
- Rechazado: escenario nuevo en `openspec/specs/` — ticket TEST + DOC, sin spec id; el catálogo ya mapea WRONGPASS.
- Rechazado: cubrir `\r\n` u otro whitespace — el repro es `"secret\n"`; Out of scope.
- Rechazado: portar el manejo de Traefik / go-redis — fuera de este cliente.
- Live contract: catálogo AUTH/WRONGPASS existente; este ticket no añade delta. Propose transcribe TEST + DOC, no inventa un spec id.

## Open questions

- Q: ¿Se recorta `Config.Pass` (o `User`) antes de AUTH para absorber el newline del Secret de Kubernetes?
  Rank: additive asked — el trabajo añade GoDoc y test, no reshapes `authArgs`; Design: «No behavior change (trimming a password would be a silent security-relevant transform)»
  Decision: resolved — no recortar; AUTH envía `Pass` byte a byte. El test de control (`"secret"`) autentica; `"secret\n"` es `redis:noauth`.
  By: explore

- Q: ¿Propose añade un escenario al catálogo `openspec/specs/` además del GoDoc?
  Rank: additive incidental — un escenario nuevo sería un contrato añadido; el ticket es TEST + DOC y no nombra spec id (Unknowns; Out of scope inferido)
  Decision: assumed — no escenario de catálogo; Live WRONGPASS ya promete el mapeo; la superficie DOC es el GoDoc de `Config.Pass`.
  By: explore

- Q: ¿Un Secret o fichero con `\r\n` (Windows) debe cubrirse en el test?
  Rank: additive incidental — casos extra; Out of scope: whitespace distinto del `\n` del test de referencia; Tests nombra `"secret\n"`
  Decision: assumed — solo `"secret\n"` como el repro de referencia.
  By: explore

- Q: ¿Qué hace Traefik / go-redis con el mismo Secret?
  Rank: additive incidental — investigación de terceros extra; Out of scope: cambiar AUTH en Redis/Dragonfly o Traefik upstream
  Decision: assumed — fuera de este cliente; citar https://github.com/traefik/traefik/issues/12078; no portar trim de Traefik ni go-redis.
  By: explore

- Q: ¿El test se copia tal cual o se reescriben helpers con sufijo 12078?
  Rank: additive asked — Tests: copy as is de `TestTraefik12078_PasswordWithTrailingNewline`; helpers existentes `runForEachLiveEngine` / `newTestRedis`; no ficheros de helpers compartidos nuevos
  Decision: resolved — copiar `simpleredis/traefik_issue12078_e2e_test.go` tal cual; reusar el harness del paquete; no duplicar helpers ni crear ficheros compartidos nuevos.
  By: explore

- Q: ¿El paquete de uso `std_go_simpleredis.md` debe ganar un Gotcha de newline, o basta el GoDoc de `Config.Pass`?
  Rank: additive incidental — edición extra de usage; Design nombra GoDoc en `Config.Pass`
  Decision: assumed — DOC requerida = GoDoc de `Config.Pass` (byte a byte, sin recorte, newline de Secret/fichero hace fallar AUTH, cita #12078). El usage packet no es la superficie nombrada.
  By: explore

- Q: ¿Se añade una fila fake `"secret\n"` en `TestAuthArgumentForms` para el job Unit `-short`?
  Rank: additive incidental — caso unit extra; Tests nombra solo la copia live
  Decision: assumed — solo el e2e live copiado; Unit `-short` se queda como hoy. El envío es `[]byte(sr.pass)` sin trim.
  By: explore

- Q: ¿El GoDoc de `Config.User` también debe decir que no se recorta?
  Rank: additive incidental — doc de otro campo; Design nombra `Config.Pass`; Out of scope recortar `User`
  Decision: assumed — GoDoc solo en `Config.Pass`; `User` se queda como está.
  By: explore
