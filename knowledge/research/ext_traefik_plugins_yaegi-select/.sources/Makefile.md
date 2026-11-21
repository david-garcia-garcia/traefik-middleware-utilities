---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/82498271f7f15b4aa128bb3e13fc015e40242e6f/Makefile
title: crowdsec-bouncer-traefik-plugin Makefile test targets
fetched: 2026-11-21
authority: source
ref: github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin@82498271f7f15b4aa128bb3e13fc015e40242e6f:Makefile
---

```
test:
	go test -v -cover ./...

yaegi_test:
	yaegi test -v .
```

`yaegi_test` interprets the root package only; `pkg/...` tests (including the Yaegi poll-timer test) run under `go test`.
