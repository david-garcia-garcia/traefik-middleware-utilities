# Deviations

- [x] taken  AUTH/SELECT live instead of fake-TCP-only
  Asked: do not port AUTH/SELECT; live-e2e SHALL keep AUTH and SELECT fake-TCP only.
  Instead: keep DestBranch SELECT 99 and WRONGPASS in `pool_e2e_test.go`; passworded AUTH env on the e2e job.
  Owner: `simpleredis/pool_e2e_test.go`
  Why: honouring the letter would drop dest’s live handshake proof when merging `origin/master`.
  By: implement
  Requester: not asked
