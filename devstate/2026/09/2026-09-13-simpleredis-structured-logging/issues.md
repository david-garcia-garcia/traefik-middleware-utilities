# Issues

- [x] note large  `knowledge/debt/2026-09-13-simpleredis-leftover-resp-log-events.md`
  Why: `MsgSocketPoisoned` and `MsgAuthLeftover` have no dest detection site until PR #69 merges.
  Taken: PR #69 is on dest after Sync; both events fire at the leftover checks in `do`.
