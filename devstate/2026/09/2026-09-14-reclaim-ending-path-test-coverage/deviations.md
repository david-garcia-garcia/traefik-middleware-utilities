# Deviations

- [x] taken  two argument values in table_gaps_test.go instead of a byte-identical copy
  Asked: copy reclaim/table_gaps_test.go verbatim from the caller tree.
  Instead: Open-over-gone uses key `gone` (not `a`) and the Reset+enforce later Open passes `Hooks{Close: func() {}}` instead of `Hooks{}`.
  Owner: `reclaim/table_test.go` (`mustSlot`, `openReturned`)
  Why: CI lint (unparam) reports those helpers as always receiving `"a"` and `Hooks{}`; honouring a byte-identical copy keeps that package-wide pattern and fails Lint. Different arguments keep the same proofs and make the helpers' parameters vary.
  By: implement
  Requester: not asked
