# Standards

1. [judgement] Mysterious Name — `simpleredis/simpleredis.go:46` — field `buf` is the per-conn encode scratch (`maxIdleEncodeBuf`, `appendRESP`, idle trim); the identifier is a generic byte slice, not that role
   → Rename the field to `scratch` (or `encodeBuf`) so the type’s idle-list job is in the name
   Status: done
   Argument: moot. The encoder was reduced to `bufio.Writer` plus `strconv.AppendInt`, so `pooledConn.buf` and `maxIdleEncodeBuf` no longer exist. What remains is `pooledConn.lenBuf`, named for the one thing it holds.
