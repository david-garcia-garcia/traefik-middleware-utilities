# go-redis under Yaegi

Whether `github.com/redis/go-redis/v9` can run inside the Yaegi interpreter Traefik uses for plugins, with and without `useUnsafe`. This is the standing justification for `simpleredis/` existing at all. Pin: yaegi v0.16.1 (this module and Traefik v3.7.11), interpreter built as Traefik v3.7.11 `newInterpreter` builds it.

## It does not run, at any symbol set

`useUnsafe` changes *where* it fails, not *whether*. Measured on Linux with `interp.New(interp.Options{GoPath: goPath})` plus `stdlib.Symbols`, adding `unsafe.Symbols` and `syscall.Symbols` for the useUnsafe rows, against a `redis:7-alpine` peer:

| go-redis | symbols | outcome |
| --- | --- | --- |
| v9.22.0 | stdlib only | import fails: `unable to find source related to: "unsafe"` |
| v9.22.0 | + unsafe + syscall | type-check fails: `package unsafe "unsafe" has no symbol String` |
| v9.11.0 | stdlib only | import fails: `unable to find source related to: "unsafe"` |
| v9.11.0 | + unsafe + syscall | imports and type-checks OK, then **segfaults the process on the first PING** |
| any | compiled control | `ok` — PING, SET, GET, miss-is-`redis.Nil`, INCR, INCRBY, MGET, pipeline, HSET/HGETALL, EVAL all correct |

The compiled control ran the identical source file, staged into the GOPATH tree by copy, so the interpreted and compiled paths cannot diverge. No go-redis source was stubbed, shimmed, or patched on any row.

## Without useUnsafe: `unsafe` has no source

go-redis reaches `unsafe` through `internal/util`, and Yaegi has no source for it:

```
import "github.com/redis/go-redis/v9" error: .../internal/arg.go:8:2:
import "github.com/redis/go-redis/v9/internal/util" error: .../internal/util/unsafe.go:6:2:
import "unsafe" error: unable to find source related to: "unsafe"
```

At v9.22.0 the same failure arrives one hop earlier, through `go.uber.org/atomic/unsafe_pointer.go`.

## With useUnsafe: current go-redis needs symbols Yaegi lacks

`internal/util/unsafe.go` at v9.22.0 is `unsafe.String(unsafe.SliceData(b), len(b))` / `unsafe.Slice(unsafe.StringData(s), len(s))`, and Yaegi exports only `Pointer`, `Add`, `Sizeof`, `Alignof`, `Offsetof` (`ext_traefik_plugins_yaegi-unsafe/`), so this fails at type-check: `.../internal/util/unsafe.go:11:9: package unsafe "unsafe" has no symbol String`.

This is a **recent regression in go-redis's interpretability**: every v9 up to v9.15.0 used the older `*(*string)(unsafe.Pointer(&b))` cast, which Yaegi does support. ([go-redis@v9.22.0:internal/util/unsafe.go](https://github.com/redis/go-redis/blob/v9.22.0/internal/util/unsafe.go), [.sources/unsafe.go.md](.sources/unsafe.go.md))

## The furthest it gets: full load, then memory corruption

v9.11.0 with `stdlib+unsafe+syscall` needs only `unsafe.Pointer`. The whole library imports in 0.38s and type-checks, and a no-I/O function that calls `redis.NewClient(&redis.Options{...})` and `redis.NewStringCmd(...)` returns `ok`. The first real operation then dies:

```
STAGE=import OK (0.383s)
STAGE=eval goredisprobe.TypesOnly() => ok
STAGE=eval goredisprobe.Minimal("...:6379") FAIL: runtime error: invalid memory address or nil pointer dereference
```

The fault is inside Yaegi, not in go-redis logic:

```
runtime.memmove() / reflect.packEface / reflect.valueInterface / reflect.Value.Interface
github.com/traefik/yaegi/interp.getMethodByName.func1
 /go/pkg/mod/github.com/traefik/yaegi@v0.16.1/interp/run.go:1960
```

The interpreted frame chain is `Minimal → Ping → Process → processHook → process → _process → withConn → getConn`, innermost at `redis.go:248` (`if c.opt.Limiter != nil`). The failing `reflect.Value` carries a valid type word and a garbage data pointer (`unexpected fault address 0x18005b56f700`), so Yaegi's frame data is already corrupted by the time `getConn` runs. On a second eval the process did not merely panic but died — `fatal error: fault`, unrecoverable. **In a Traefik plugin that kills the proxy, not just the middleware.**

That language shape is not on its own the cause: a standalone interpreted package with a nil interface field in a pointed-to struct, the `!= nil` guard, and a guarded dispatch all return `ok` under both symbol sets. The smallest reproducer reached is importing unmodified go-redis v9.11.0 with `stdlib+unsafe+syscall` and calling `client.Ping(ctx)`; it could not be reduced below the full library. So there is no single missing Yaegi feature whose arrival would fix this.

## Collateral facts

- Even the partial load is expensive. Staging v9.11.0 is 72 `.go` files / 690 KB across three modules (go-redis, `cespare/xxhash/v2`, `dgryski/go-rendezvous`); v9.22.0 needs 571 files / 10.5 MB because it pulls `go.uber.org/atomic` and all of `golang.org/x/sys`. Import cost 0.38s at v9.11.0 against 1.3 ms for the compiled control to run the *entire* operation sequence. Under Traefik every plugin instantiation would pay that import.
- go-redis v9.22.0 declares `go 1.24`; this repo is `go 1.21`.
- go-redis v9.15.0 does not compile on Windows at all (`internal/pool/pool.go:809: undefined: errUnexpectedRead`, because that symbol lives only in the Unix-only `conn_check.go`). Unrelated to Yaegi, fixed by v9.22.0, but it makes that release unusable for local dev on Windows.
- Yaegi ignores `//go:build`, so even a working interpreted go-redis would silently lose `conn_check.go` to `conn_check_dummy.go` and run with no socket liveness probe (`ext_traefik_plugins_yaegi-build-constraints/`).
- `useUnsafe` is a dual gate: manifest **and** operator settings, where manifest-true with operator-false makes Traefik refuse to load the plugin (`ext_traefik_plugins_useunsafe/`). This repo's compose sets it false for both local plugins.

Product consequence: `knowledge/devdocs/std_go_simpleredis.md` (Language, Gotchas).

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| v9.22.0 `internal/util` uses `unsafe.String` / `unsafe.Slice` | go-redis@v9.22.0 internal/util/unsafe.go | source |
| Yaegi exports only Pointer/Add/Sizeof/Alignof/Offsetof from `unsafe` | yaegi@v0.16.1 stdlib/unsafe (`ext_traefik_plugins_yaegi-unsafe/`) | source |
| Fault is inside `interp.getMethodByName` at run.go:1960 | yaegi@v0.16.1 interp/run.go, captured stack | source |
| No symbol set runs a single Redis command; v9.11.0 segfaults on first PING | measured against yaegi v0.16.1 with a live redis:7-alpine peer | inference |
| Compiled control passes every operation on identical source | same probe | inference |
| Staging size and import timings | same probe | inference |
