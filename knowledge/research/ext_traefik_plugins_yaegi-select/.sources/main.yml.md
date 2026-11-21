---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/82498271f7f15b4aa128bb3e13fc015e40242e6f/.github/workflows/main.yml
title: crowdsec-bouncer-traefik-plugin CI workflow
fetched: 2026-11-21
authority: source
ref: github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin@82498271f7f15b4aa128bb3e13fc015e40242e6f:.github/workflows/main.yml
---

Job `main`: `GO_VERSION: 1.22`, `YAEGI_VERSION: v0.16.1` ("pin to the version current Traefik bundles"). Steps include: install Yaegi via `traefik/yaegi` `install.sh` into `$(go env GOPATH)/bin`; `make lint`; `make test`; copy vendored `traefik-middleware-utilities` onto `GOPATH/src` ("Yaegi v0.16 resolves imports from GOPATH/src, not vendor"); `make yaegi_test` with `GOPATH` set.

Job `race`: `CGO_ENABLED: 1`, runs `go test -race -count=1 -p 1 ./pkg/...` ("Parallel packages would race those registries"). This job does not install Yaegi.
