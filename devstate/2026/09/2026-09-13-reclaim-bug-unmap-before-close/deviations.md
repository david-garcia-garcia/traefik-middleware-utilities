# Deviations

- [x] taken  `TestTable_` names instead of the ticket’s example identifier
  Asked: the zero-grace overlap test is named `TestZeroGraceCreateStartsWhilePreviousCloseBlocked`.
  Instead: `TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns` (and `TestTable_ExpireCreateWaitsUntilPreviousCloseReturns` for expire).
  Owner: `reclaim/table_test.go`
  Why: every compiled table test already uses the `TestTable_` prefix; the passing invariant is wait-until-Close, not the dest overlap the example name describes.
  By: explore
  Requester: not asked
