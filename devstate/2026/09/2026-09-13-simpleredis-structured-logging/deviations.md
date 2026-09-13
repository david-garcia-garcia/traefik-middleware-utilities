# Deviations

- [x] taken  discard logger at New instead of nil-silent and no catalog
  Asked: optional `Config.Logger`, nil stays silent, do not install a discard handler; exported `Msg*` constants and nil-safe helpers in `log.go`; extra attrs (dial reason, short-bulk announced/read, Open knobs).
  Instead: `New` assigns a discard logger when Logger is nil; call sites log inline `simpleredis_*` strings with no constants, no `log.go`, and no extra capture types.
  Owner: `simpleredis/simpleredis.go` `New`
  Why: honouring the catalog and nil-checks added a logging facade and control-flow changes whose only job was attributes.
  By: implement
  Requester: confirmed
