# Traefik local plugin loader (Yaegi)

How Traefik v3 loads a **local** middleware plugin through Yaegi: directory layout, manifest, static config, and the symbols Traefik evaluates at startup. Geoblock @ `22f09a0` is the measured reference layout.

## GOPATH tree, not `go mod` fetch

Yaegi runs plugin source from a GOPATH workspace. Traefik's constant is `./plugins-local/` relative to the Traefik process working directory ([traefik:pkg/plugins/plugins.go](https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/plugins.go)). Plugin sources live at:

```
plugins-local/src/<module import path>/
```

Geoblock compose bind-mounts the repo root there:

```
./:/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock
```

([geoblock@22f09a0:docker-compose.yml](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/docker-compose.yml))

Official plugindemo matches: local plugins under `./plugins-local/src/<module>/` ([plugindemo readme](https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md)). Yaegi itself does not support Go modules for dependency resolution; third-party deps must be **vendored** in the plugin tree ([yaegi README](https://github.com/traefik/yaegi/blob/v0.16.1/README.md)).

A plugin module may still carry `go.mod` for local `go test` / lint (geoblock has one). Traefik's loader does not run `go mod download`; it interprets from GOPATH + vendored trees.

Extracts: [.sources/plugindemo-readme.md](.sources/plugindemo-readme.md), [.sources/plugins.go.md](.sources/plugins.go.md), [.sources/docker-compose.yml.md](.sources/docker-compose.yml.md)

## Static config and manifest

**Static** (Traefik startup only):

```yaml
experimental:
  localPlugins:
    geoblock:                              # alias used in dynamic middleware labels
      moduleName: github.com/david-garcia-garcia/traefik-geoblock
      settings:
        useunsafe: true                    # enables Yaegi unsafe/syscall symbols
```

CLI equivalent in geoblock compose:

```
--experimental.localplugins.geoblock.modulename=github.com/david-garcia-garcia/traefik-geoblock
--experimental.localplugins.geoblock.settings.useunsafe=true
```

**Manifest** at plugin root (`.traefik.yml`):

```yaml
displayName: geoblock
type: middleware
import: github.com/david-garcia-garcia/traefik-geoblock
useUnsafe: true
```

Traefik reads the manifest from `plugins-local/src/<moduleName>/.traefik.yml`. For Yaegi plugins, `import` must be non-empty and must be a **prefix of** `moduleName` ([traefik:plugins.go](https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/plugins.go)). Usually both are the module root.

**Dynamic** middleware (Docker labels example):

```
traefik.http.middlewares.geoblock2.plugin.geoblock.mode=enrichandblock
```

The label segment after `plugin.` is the **static alias** (`geoblock`), not the Go import path.

Extracts: [.sources/traefik.yml.md](.sources/traefik.yml.md), [.sources/docker-compose.yml.md](.sources/docker-compose.yml.md), [.sources/plugins.go.md](.sources/plugins.go.md)

## Loader sequence at startup

At startup, for each `localPlugins` entry Traefik ([traefik:middlewareyaegi.go](https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/middlewareyaegi.go)):

1. Builds Yaegi with `interp.Options{GoPath: "./plugins-local/"}` and stdlib (+ unsafe if `useunsafe`).
2. `Eval import "<manifest.import>"` — only the manifest import path; subpackages load when the root package imports them.
3. Resolves `basePkg`: manifest `basePkg`, or last import segment with `-` → `_` (e.g. `traefik-geoblock` → `traefik_geoblock`).
4. `Eval basePkg.New` and `Eval basePkg.CreateConfig`.

Per applied dynamic config, Traefik calls `CreateConfig`, decodes plugin settings into it, then `New(ctx, next, config, middlewareName)`. No plugin `Close`/`Stop` ([ext_traefik_plugins_middleware-lifecycle](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/research/ext_traefik_plugins_middleware-lifecycle/notes.md) in geoblock — not duplicated here).

Extracts: [.sources/middlewareyaegi.go.md](.sources/middlewareyaegi.go.md)

## Required plugin entry symbols

Root package (geoblock: `package traefik_geoblock` in `plugin.go`):

| Symbol | Signature | Role |
|--------|-----------|------|
| `Config` | struct or type alias | Decoded from dynamic config |
| `CreateConfig` | `func CreateConfig() *Config` | Default config factory |
| `New` | `func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error)` | Middleware constructor |

Geoblock aliases `Config` to `pkg/geoblock.Config` and delegates `CreateConfig`/`New` to subpackages; subpackages (`pkg/reclaim`, `pkg/geoblock`) resolve through GOPATH when the root imports them ([ext_traefik_plugins_yaegi-subpackages](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/research/ext_traefik_plugins_yaegi-subpackages/notes.md) in geoblock).

For this spin-off (`traefik-middleware-utilities`): mount repo at `plugins-local/src/github.com/david-garcia-garcia/traefik-middleware-utilities`, expect `basePkg` `traefik_middleware_utilities`, root `plugin.go` with the three exports, `reclaim/` as an importable subpackage.

Extracts: [.sources/plugin.go.md](.sources/plugin.go.md), [.sources/plugindemo-readme.md](.sources/plugindemo-readme.md)

## Docker image and unsafe

Geoblock integration stack pins **`traefik:v3.7.11`** (Yaegi v0.16.1). `useunsafe: true` is required for geoblock's IP2Location reader and for interpreted code that needs `unsafe` symbols; reclaim table itself is stdlib-only but the harness should match geoblock's pin.

Extracts: [.sources/docker-compose.yml.md](.sources/docker-compose.yml.md)

## Yaegi constraints from reclaim specs (loader-relevant only)

From geoblock reclaim specs @ `22f09a0` (full lifecycle semantics are product specs, not repeated here):

- `create` passed to `Open` must be `func() (any, error)` — Yaegi cannot call `func(context.Context) (any, error)` ([std_go_reclaim_context-lease spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/openspec/specs/std_go_reclaim_context-lease/spec.md)).
- Shared package must use non-generic `Table` storing `any`; cross-package `Table[T]` fails under Yaegi ([same spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/openspec/specs/std_go_reclaim_context-lease/spec.md), [ext_traefik_plugins_yaegi-generics/](../ext_traefik_plugins_yaegi-generics/notes.md) in this repo).

Extracts: [.sources/std_go_reclaim_context-lease-spec.md](.sources/std_go_reclaim_context-lease-spec.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Local plugins under `./plugins-local/src/<module>/` | plugindemo README | official |
| Export `Config`, `CreateConfig`, `New(ctx, next, config, name)` | plugindemo README | official |
| Third-party deps vendored; Yaegi no Go modules | plugindemo + yaegi README | official |
| `localGoPath = "./plugins-local/"` | traefik@v3.7.11:plugins.go | source |
| Manifest `import` prefix of `moduleName` | traefik@v3.7.11:plugins.go | source |
| Eval import + basePkg.New/CreateConfig | traefik@v3.7.11:middlewareyaegi.go | source |
| Geoblock compose mount + static localPlugins | geoblock@22f09a0:docker-compose.yml | source |
| Geoblock `.traefik.yml` + plugin.go exports | geoblock@22f09a0 | source |
| Image `traefik:v3.7.11` | geoblock@22f09a0:docker-compose.yml | source |
| `create func() (any, error)` Yaegi constraint | geoblock@22f09a0:std_go_reclaim_context-lease | source |

## References

- https://plugins.traefik.io/create
- https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md
- https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/plugins.go
- https://github.com/traefik/traefik/blob/v3.7.11/pkg/plugins/middlewareyaegi.go
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/docker-compose.yml
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/.traefik.yml
- https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/plugin.go
