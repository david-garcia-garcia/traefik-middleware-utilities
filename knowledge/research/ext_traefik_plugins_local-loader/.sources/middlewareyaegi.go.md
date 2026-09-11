---
url: https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/middlewareyaegi.go
title: pkg/plugins/middlewareyaegi.go
fetched: 2026-09-11
authority: source
ref: github.com/traefik/traefik@v3.7.11:pkg/plugins/middlewareyaegi.go
---

`newInterpreter`: `interp.Options{GoPath: goPath, Env: os.Environ(), ...}`; loads stdlib and optionally unsafe/syscall.

Then: `i.Eval(fmt.Sprintf(\`import "%s"\`, manifest.Import))`.

`newYaegiMiddlewareBuilder`:
- If `basePkg == ""`: `basePkg = strings.ReplaceAll(path.Base(imp), "-", "_")`.
- `i.Eval(basePkg + ".New")`, `i.Eval(basePkg + ".CreateConfig")`.

`New` invoked as `(ctx, next, config, middlewareName)`; must return `http.Handler`.
`CreateConfig` invoked with no args; result mapstructure-decoded from dynamic plugin config.

Only manifest import is Eval-imported; subpackages load via root package imports.
