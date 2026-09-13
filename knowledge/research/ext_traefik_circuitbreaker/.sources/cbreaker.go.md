---
url: https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/cbreaker.go
title: cbreaker/cbreaker.go
fetched: 2026-09-12
authority: source
ref: vulcand/oxy@07821e22d8655dcce7d23d1038a8d325e5dc234b:cbreaker/cbreaker.go
---

New default durations before options: checkPeriod 100ms, fallbackDuration 10s, recoveryDuration 10s; defaultFallback writes 503.
States: standby (Closed), tripped (Open), recovering. Zero value is standby.
Tripped: fallback until until; then recovering. Recovering: ratioController may pass; after until → standby. checkAndSet on every served response, but real eval at most every checkPeriod; skip if already tripped; condition true → tripped + metrics.Reset().
parseExpression(expression) must succeed or New fails.
