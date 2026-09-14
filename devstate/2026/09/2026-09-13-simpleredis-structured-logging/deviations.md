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

- [x] taken  a peer reply is published as its leading error code, not "peer text except on AUTH"
  Asked: log the peer's reply text everywhere except when the command in flight is AUTH, where the site logs the mapped sentinel instead (a different rule allowed if simpler and enforced by code).
  Instead: `loggableCause`, the one sink every site line goes through, prints an error this package owns as itself and any other error as the leading `A`-`Z` token of the peer's text only (`ERR`, `LOADING`, `NOSCRIPT`); a leading token that is not all `A`-`Z` is not published at all.
  Owner: `simpleredis/simpleredis.go` `loggableCause`
  Why: the verb rule protects `Pass` but not keys. Redis refuses an unknown verb by quoting the command's own arguments back (`ERR unknown command 'MSETEX', with args beginning with: '2', '<key>', '<value>'`), which the same spec already bans, and that reply is not on the AUTH path so a verb gate would publish it. One sink also means no site, level, or verb can bypass the rule, and `LOADING` / `MISCONF` / `NOSCRIPT` stay visible. Measured: publishing the raw text fails `TestLogHandshakeFailedNeverEchoesPass` on the password and `TestLogPeerReplyIsCodeOnlyNotItsArguments` on the key name.
  By: implement
  Requester: not asked

- [x] taken  eight simpleredis_* events survive the catalog removal, not three
  Asked: fold failures into site logging; `simpleredis_dial` (with `reason`), `simpleredis_open` and `simpleredis_idle_swept` are non-errors a site-error logger cannot express, so do not drop them.
  Instead: `simpleredis_retry`, `simpleredis_capability`, `simpleredis_over_free`, `simpleredis_socket_poisoned` and `simpleredis_panic` stay named events too.
  Owner: `openspec/specs/std_go_simpleredis_slog-events/spec.md` "A condition with no error keeps a named event"
  Why: same test as the three named in the ask — none of the five has an error value to site-tag. `over_free` and `panic` are exactly where the stash invented `fmt.Errorf("over-free")` / `panic: %v` and discarded the result; `socket_poisoned` happens on a command that succeeded; `retry` and `capability` are decisions, and `retry` is the only line that says four failures were one command and not four.
  By: implement
  Requester: not asked
