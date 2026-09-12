# Standards

1. [judgement] Duplicated Code — `simpleredis/simpleredis.go:341` and `simpleredis/simpleredis.go:378` — the same `append([]byte(nil), …[1:]...)` copy is written at the scalar `+`/`:` return and again at array `:`/`+` slots
   → Extract one helper that copies a RESP status/integer payload, call it from both sites
   Status: skipped
   Argument: judgement; two-site copy is the design-named shape; extract is extra abstraction, not applied unattended.
