# Deviations

- [x] taken  type-assert the handshake mark instead of errors.As
  Asked: detect the dial mark with `errors.As` (explore assumed).
  Instead: `err.(handshakeFailure)` in `isHandshakeFailure`.
  Owner: `simpleredis/commands_exec.go`
  Why: Yaegi panics on `errors.As` for this struct (`*target must implement error`); Traefik interprets this package.
  By: implement
  Requester: not asked
