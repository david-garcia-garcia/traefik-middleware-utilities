---
url: https://github.com/golang/go/blob/go1.21.13/src/bufio/bufio.go
title: Go 1.21 bufio Reader buffer size and ReadSlice
fetched: 2026-09-11
authority: source
ref: github.com/golang/go@go1.21.13:src/bufio/bufio.go
---

`defaultBufSize = 4096` (unexported in Go 1.21). `NewReader` calls `NewReaderSize(rd, defaultBufSize)`.

`ErrBufferFull = errors.New("bufio: buffer full")`.

`ReadSlice(delim)`: search the buffer for delim; on hit return `b.buf[b.r : b.r+i+1]` (includes delim) and advance `b.r`. If `Buffered() >= len(b.buf)` before finding delim: set `line = b.buf`, `err = ErrBufferFull`. Comment: bytes stop being valid at the next read; `err != nil` iff line does not end in delim.
