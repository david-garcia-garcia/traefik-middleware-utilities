---
url: https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/types.go
title: pkg/plugins/types.go Manifest and Settings
fetched: 2026-09-11
authority: source
ref: github.com/traefik/traefik@v3.7.11:pkg/plugins/types.go
---

`Settings.UseUnsafe bool` — json/toml/yaml `useUnsafe`; description "Allow the plugin to use unsafe and syscall packages."

`Manifest.UseUnsafe bool` — yaml `useUnsafe`.

`LocalDescriptor.Settings` is the static local-plugin settings block (`experimental.localPlugins.<alias>.settings`).
