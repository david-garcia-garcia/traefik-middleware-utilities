---
url: https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/memmetrics/roundtrip.go
title: memmetrics/roundtrip.go
fetched: 2026-09-12
authority: source
ref: vulcand/oxy@07821e22d8655dcce7d23d1038a8d325e5dc234b:memmetrics/roundtrip.go
---

NetworkErrorRatio = netErrors / total; 0 if total is 0.
Record increments netErrors only for HTTP 504 Gateway Timeout and 502 Bad Gateway.
ResponseCodeRatio(startA,endA,startB,endB) = count(code in [startA,endA)) / count(code in [startB,endB)); 0 if denominator 0.
