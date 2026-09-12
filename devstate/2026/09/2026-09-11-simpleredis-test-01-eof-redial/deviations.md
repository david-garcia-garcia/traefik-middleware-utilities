# Deviations

- [x] taken  CLIENT KILL ADDR of LocalAddr, then ID if the engine reports 0
  Asked: sidecar `CLIENT KILL ADDR` of `sr.idle[0].netConn.LocalAddr()` only.
  Instead: try ADDR first; if kill count is 0, `CLIENT ID` on that pooled socket then `CLIENT KILL ID`.
  Owner: `simpleredis/live_test.go`
  Why: honouring ADDR-only fails when Docker or GitHub Actions port-publish NAT hides LocalAddr from Redis/Dragonfly; ID still kills only that pooled conn so parallel CI clients survive.
  By: implement
  Requester: not asked
