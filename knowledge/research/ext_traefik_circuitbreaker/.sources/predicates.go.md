---
url: https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/predicates.go
title: cbreaker/predicates.go
fetched: 2026-09-12
authority: source
ref: vulcand/oxy@07821e22d8655dcce7d23d1038a8d325e5dc234b:cbreaker/predicates.go
---

parseExpression uses vulcand/predicate with AND/OR/EQ/NEQ/LT/LE/GT/GE and functions LatencyAtQuantileMS, NetworkErrorRatio, ResponseCodeRatio.
LatencyAtQuantileMS(quantile float64) → histogram latency in ms (int); histogram error → 0.
NetworkErrorRatio / ResponseCodeRatio wrap memmetrics.
Operators map to && || == != < <= > >=. Empty string is not special-cased here; Parse of the raw string must yield an hpredicate.
