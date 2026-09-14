# Deviations

- [x] taken  discard logger at New instead of nil-silent and no catalog
  Asked: optional `Config.Logger`, nil stays silent, do not install a discard handler; exported `Msg*` constants and nil-safe helpers in `log.go`; extra attrs (dial reason, short-bulk announced/read, Open knobs).
  Instead: `New` assigns a discard logger when Logger is nil; call sites log inline `simpleredis_*` strings with no constants, no `log.go`, and no extra capture types.
  Owner: `simpleredis/simpleredis.go` `New`
  Why: honouring the catalog and nil-checks added a logging facade and control-flow changes whose only job was attributes.
  By: implement
  Requester: confirmed

- [x] taken  simpleredis_timeout emits in exec, not in do
  Asked: re-place this branch's log call sites onto the new code shapes.
  Instead: `libraryTimeout` became a method on `*SimpleRedis` and emits the event; `do` no longer emits it.
  Owner: `simpleredis/commands_exec.go` `libraryTimeout`
  Why: PR #84 made `ioOrContext` hand the socket deadline up as a context deadline so `exec` decides library budget versus caller deadline. Keeping the emit in `do` left the event unreachable on the exec path — measured: disabling the `exec` emit fails `TestLogTimeout`, and the captured dump at that point held only `simpleredis_open` and `simpleredis_dial`.
  By: mergeconflictresolve
  Requester: pending

- [x] taken  simpleredis_handshake_failed carries verb, not the peer error text
  Asked: never log the value of `Config.Pass`.
  Instead: the event's `error` attribute became `verb` (`AUTH` / `SELECT`); the peer text stays on the returned error only.
  Owner: `simpleredis/pool.go` `dial`
  Why: Redis 7.4 answers AUTH against a nopass default user with `ERR AUTH <password> called without any password configured`, which is not an AUTH-class prefix, so that branch logged the password. Measured with a fake peer before the fix. `TestLogHandshakeFailedNeverEchoesPass` now pins it.
  By: mergeconflictresolve
  Requester: pending
