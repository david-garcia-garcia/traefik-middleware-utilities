# Deviations

- [x] taken  warm the drop client before Eval as well as before Incr
  Asked: warm the drop client with a pass-through Get, then Incr and Eval through the drop client.
  Instead: warm, Incr, then warm again, then Eval.
  Owner: `e2e/simpleredisprobe/plugin.go`
  Why: honouring warm-once would put Eval on a new dial, which dest already does not retry; the live proof would not pin the verb gate.
  By: implement
  Requester: not asked
