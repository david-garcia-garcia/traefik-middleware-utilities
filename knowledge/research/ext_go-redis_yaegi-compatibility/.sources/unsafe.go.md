---
url: https://github.com/redis/go-redis/blob/v9.22.0/internal/util/unsafe.go
title: internal/util/unsafe.go
fetched: 2026-09-13
authority: source
ref: github.com/redis/go-redis@v9.22.0:internal/util/unsafe.go
---

Whole file, constrained `//go:build !appengine`:

```go
package util

import (
	"unsafe"
)

// BytesToString converts byte slice to string.
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes converts string to byte slice.
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
```

Four symbols Yaegi v0.16.1 does not export: `String`, `SliceData`, `Slice`, `StringData`. This is the file the interpreted import dies on — without unsafe symbols on the `import "unsafe"` line, with unsafe symbols on `unsafe.String` at line 11.

`internal/util` is reached from `internal/arg.go`, so it is on the import path of the root `redis` package; it cannot be avoided by not calling those helpers.

Versions up to and including v9.15.0 implemented the same two functions with `*(*string)(unsafe.Pointer(&b))` and a string-header cast, which need only `unsafe.Pointer` — a symbol Yaegi does export.
