---
url: https://pkg.go.dev/os#ErrDeadlineExceeded
title: os.ErrDeadlineExceeded
fetched: 2026-09-13
authority: official
---

`var ErrDeadlineExceeded = errDeadlineExceeded() // "i/o timeout"`

Portable analog of a common system-call error. Test with `errors.Is`.

`os.File.SetDeadline` restates the same contract as `net.Conn`: if the deadline is exceeded, Read/Write return an error that wraps `ErrDeadlineExceeded`; `errors.Is(err, os.ErrDeadlineExceeded)`; that error implements `Timeout()` returning true.
