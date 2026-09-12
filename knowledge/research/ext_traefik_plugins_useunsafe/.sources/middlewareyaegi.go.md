---
url: https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/middlewareyaegi.go
title: pkg/plugins/middlewareyaegi.go newInterpreter
fetched: 2026-09-11
authority: source
ref: github.com/traefik/traefik@v3.7.11:pkg/plugins/middlewareyaegi.go
---

`newInterpreter` loads `stdlib.Symbols` always.

Then:

```
if manifest.UseUnsafe && !settings.UseUnsafe {
  return nil, errors.New("this plugin uses restricted imports. If you want to use it, you need to allow useUnsafe in the settings")
}

if settings.UseUnsafe && manifest.UseUnsafe {
  i.Use(unsafe.Symbols)
  i.Use(syscall.Symbols)
}
```

Then `i.Eval(import "<manifest.Import>")`.
