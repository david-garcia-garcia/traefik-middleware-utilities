# Deviations

- [x] taken  one window-math helper for Take and Peek instead of a second copy of the formula
  Asked: Peek uses the same formula and Redis keys as Take (could be a second inline preamble).
  Instead: extract the window-start, keys, weight, and TTL computation both call.
  Owner: `windowcounter/limiter.go`
  Why: honouring a second copy of `current + previous × (1 − elapsed/window)` would add a variation point to the unit whose job is that one formula.
  By: explore
  Requester: not asked
