---
url: https://pkg.go.dev/bufio@go1.21.13#Reader.ReadSlice
title: bufio.Reader.ReadSlice
fetched: 2026-09-11
authority: official
---

ReadSlice reads until the first occurrence of delim in the input, returning a slice pointing at the bytes in the buffer. The bytes stop being valid at the next read.

If ReadSlice encounters an error before finding a delimiter, it returns all the data in the buffer and the error itself (often io.EOF).

ReadSlice fails with error ErrBufferFull if the buffer fills without a delim.

Because the data returned from ReadSlice will be overwritten by the next I/O operation, most clients should use Reader.ReadBytes or ReadString instead.

ReadSlice returns err != nil if and only if line does not end in delim.
