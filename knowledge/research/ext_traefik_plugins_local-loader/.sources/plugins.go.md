---
url: https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/plugins.go
title: pkg/plugins/plugins.go
fetched: 2026-09-11
authority: source
ref: github.com/traefik/traefik@v3.7.11:pkg/plugins/plugins.go
---

`localGoPath = "./plugins-local/"`.

Local plugin manifest read via `ReadManifest(localGoPath, descriptor.ModuleName)`.

Yaegi local plugin validation:
- `import` must be non-empty.
- `import` must satisfy `strings.HasPrefix(m.Import, descriptor.ModuleName)` (import is prefix of moduleName — typically equal at module root).

Error text: `the import %q must be related to the module name %q`.
