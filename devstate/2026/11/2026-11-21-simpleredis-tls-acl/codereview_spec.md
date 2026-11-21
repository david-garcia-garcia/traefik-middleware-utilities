# Spec

1. [wrong] Expiración del contexto del comando durante el handshake — `openspec/changes/simpleredis-tls-acl/specs/std_go_simpleredis_tls-handshake/spec.md:60` — el requisito `A slow or silent TLS peer stays redis:unreachable` dice que "a `DialTimeout` or command-context expiry during the TLS handshake ... SHALL return `redis:unreachable`", pero `dial` (`simpleredis/pool.go:265`) devuelve el error del contexto cuando el contexto del comando ha vencido y `exec` lo convierte en `redis:timeout`, igual que en TCP plano y como exige el requisito vivo no modificado `Whole command has an overall deadline` (`openspec/specs/std_go_simpleredis_tcp-session/spec.md:510`); el código es coherente con la línea base y la frase del delta contradice ambos
   Fix: Corregir la frase del delta para que solo `DialTimeout` (y los demás fallos de dial no clasificados) den `redis:unreachable`, y que el vencimiento de `CommandTimeout` durante el handshake dé `redis:timeout` como en TCP plano; no cambiar el código
   Status: done
   Argument: Corregido el delta `std_go_simpleredis_tls-handshake/spec.md`: solo `DialTimeout` y fallos de dial no clasificados dan `redis:unreachable`; el vencimiento de `CommandTimeout` en el handshake da `redis:timeout` como en TCP plano. Alineada la frase de `proposal.md`; design.md y devdoc no contenían la afirmación. `openspec validate --strict` válido. Código sin cambios.
2. [extra] Fichero de herramienta ajeno al cambio — `.cursorignore:1` — fichero nuevo de configuración del editor que ningún requisito, tarea ni el apartado Impact de `proposal.md` nombra
   Fix: Quitar `.cursorignore` de esta rama
   Status: done
   Argument: Borrado `.cursorignore` de la rama.
3. [extra] Configuración de modelos del flujo de trabajo — `knowledge/opendev.json:1` — fichero nuevo con la configuración de modelos de OpenDev (`model_default`, `interaction_language`) que ningún requisito, tarea ni el apartado Impact de `proposal.md` nombra
   Fix: Quitar `knowledge/opendev.json` de esta rama
   Status: done
   Argument: Borrado `knowledge/opendev.json` de la rama.
4. [judgement] Permisos del usuario ACL en CI — `.github/workflows/ci.yml:115` — la decisión 8 de `design.md` fija `ACL SETUSER app on >apppass ~* &* +@all`; los pasos de Redis (línea 115) y Dragonfly (línea 190) omiten `&*`. El requisito `Live TLS and ACL user on Redis and Dragonfly` solo exige que exista el usuario `app`, y SimpleRedis no usa canales pub/sub, así que no cambia el comportamiento probado
   Fix: Añadir `'&*'` a ambos `ACL SETUSER`, o ajustar la decisión 8 de `design.md` a `~* +@all`
   Status: skipped
   Argument: criterio (judgement); la ejecución desatendida solo aplica hallazgos hard; `&*` no cambia el comportamiento probado (SimpleRedis no usa pub/sub); decidir entre CI y design.md queda para revisión atendida.
