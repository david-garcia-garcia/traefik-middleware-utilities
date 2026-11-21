---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/82498271f7f15b4aa128bb3e13fc015e40242e6f/pkg/yaegitest/interpreter.go
title: crowdsec-bouncer-traefik-plugin yaegitest helper
fetched: 2026-11-21
authority: source
ref: github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin@82498271f7f15b4aa128bb3e13fc015e40242e6f:pkg/yaegitest/interpreter.go
---

Runs a program under the `yaegi` v0.16.1 binary the way Traefik loads the module: GOPATH/src plus vendored `traefik-middleware-utilities`. The binary is not a module dependency.

`GoPath(t)`: temp GOPATH; symlinks (Windows: junction fallback) the module root and `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities` under `src/`.

`Run(t, goPath, source)`: `t.Skip("yaegi binary not on PATH")` when `exec.LookPath("yaegi")` fails; writes `source` to a temp `main.go`; runs `yaegi <file>` with `GOPATH=<goPath>`; non-zero exit fails the test with combined output.
