# Deviations

- [x] taken  leave `maxBulkLength` at 64 MiB instead of shrinking it to what `IOTimeout` can carry
  Asked: reconcile `maxBulkLength` with whatever the configured `IOTimeout` can actually carry so the decoder stops advertising sizes it cannot deliver.
  Instead: keep `maxBulkLength = 64 << 20`; `IOTimeout` becomes a stall bound; the overall command budget plus `watchConnClose` is the wall-time cap.
  Owner: `simpleredis/resp.go` `maxBulkLength`
  Why: honouring the letter adds a bandwidth model or a static shrink that re-breaks large values the stall fix makes readable, and would put a time-derived cap next to a parse cap that usage says must stay off Config.
  By: explore
  Requester: not asked
