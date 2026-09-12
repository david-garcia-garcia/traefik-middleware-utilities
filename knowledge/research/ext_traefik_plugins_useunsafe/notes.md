# Plugin useUnsafe load gate

Traefik v3 Yaegi plugins get `unsafe` and `syscall` interpreter symbols only when **both** the plugin manifest and the operator static settings set `useUnsafe`. One side alone does not register them. This product's Traefik pin is v3.7.11.

## Dual AND gate

`newInterpreter` always loads `stdlib.Symbols`. Then:

1. If `manifest.UseUnsafe && !settings.UseUnsafe` → return error `this plugin uses restricted imports. If you want to use it, you need to allow useUnsafe in the settings`. Traefik **refuses to load** the plugin. The import never runs.
2. If `settings.UseUnsafe && manifest.UseUnsafe` → `i.Use(unsafe.Symbols)` then `i.Use(syscall.Symbols)`.
3. Otherwise (manifest false, regardless of settings) → no unsafe/syscall symbols. A plugin that `import "unsafe"` then fails later at Eval import (`unable to find source related to: "unsafe"` in older reports), not at the restricted-imports gate.

Operator settings true and manifest false does **not** register unsafe. Manifest true and operator false is the hard refuse.

Source: [traefik@v3.7.11:pkg/plugins/middlewareyaegi.go](https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/middlewareyaegi.go) `newInterpreter`; extract [.sources/middlewareyaegi.go.md](.sources/middlewareyaegi.go.md).

## Manifest vs settings fields

`Manifest.UseUnsafe` is YAML `useUnsafe` on `.traefik.yml`. `Settings.UseUnsafe` is JSON/TOML/YAML `useUnsafe` on the plugin descriptor (catalog plugins) or local-plugin `experimental.localPlugins.<alias>.settings.useUnsafe`. Description on `Settings`: "Allow the plugin to use unsafe and syscall packages."

Source: [traefik@v3.7.11:pkg/plugins/types.go](https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/types.go); extract [.sources/types.go.md](.sources/types.go.md).

Official catalog docs: `.traefik.yml` field `useUnsafe` is optional, defaults to `false`, "Allow plugin to use `unsafe` and `syscall` packages. Security implications apply." They document the **manifest** flag, not the dual-gate refuse text.

Source: [Traefik Hub Plugin Development Guide — Manifest Fields](https://doc.traefik.io/traefik-hub/api-gateway/guides/plugin-development-guide); extract [.sources/plugin-development-guide.md](.sources/plugin-development-guide.md).

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Unsafe/syscall registered only when both flags true | traefik@v3.7.11 middlewareyaegi.go | source |
| Manifest true + settings false → restricted-imports error | traefik@v3.7.11 middlewareyaegi.go | source |
| Manifest YAML `useUnsafe`; Settings `useUnsafe` | traefik@v3.7.11 types.go | source |
| Manifest field optional, default false | Traefik Hub plugin development guide | official |
