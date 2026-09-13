---
url: https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/ratio.go
title: cbreaker/ratio.go
fetched: 2026-09-12
authority: source
ref: vulcand/oxy@07821e22d8655dcce7d23d1038a8d325e5dc234b:cbreaker/ratio.go
---

allowedRequestsRatio = 0.5 * (Now - Start) / Duration.
0.5 is the allow==deny equilibrium; after that the controller allows remaining requests (ratio 1 is unreachable while denials exist).
allowRequest admits when (allowed+1)/(allowed+denied+1) < targetRatio.
