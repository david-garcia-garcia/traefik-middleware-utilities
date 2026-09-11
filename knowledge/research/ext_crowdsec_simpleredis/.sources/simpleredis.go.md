---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/simpleredis/simpleredis.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/simpleredis/simpleredis.go
title: pkg/simpleredis SimpleRedis client
fetched: 2026-09-11
authority: source
---

Package comment: GET, MGET, SET, DELETE, TTL via timetoleave.
Imports: bufio, errors, io, net, os, strconv, strings, sync, time. No unsafe, no C, no generics.
Exported error strings: redis:unreachable, redis:miss, redis:timeout, redis:noauth, redis:issue?
Pool: maxIdleConns 8, idleTimeout 30s, dialTimeout 2s, ioTimeout 1s.
SimpleRedis fields: host, pass, database, mu, idle []*pooledConn, closed.
Init(host, pass, database) assigns fields; not mutex-protected; call once before concurrent use.
Close drains idle, sets closed; further commands return unreachable and do not dial; in-flight finish then sockets closed on release; idempotent.
Get → GET; MGet → MGET (empty names: nil,nil); Set → SET key data EX duration; Del → DEL.
dial: net.Dialer TCP to host; AUTH if pass set; SELECT if database set.
writeCommand: RESP array of bulk strings.
readReply: +/: success payload; - via replyError; $ bulk or miss; * array with nil slots on $-1.
replyError prefixes → redis:noauth: NOAUTH, WRONGPASS, NOPERM, ERR Client sent AUTH.
ioError: errors.Is os.ErrDeadlineExceeded → redis:timeout; else unreachable. Comment: do not net.Error-assert; Yaegi has panicked on that interface across the interpreter boundary.
exec retries once on a dead reused conn unless timeout or the first attempt was reusable/clean.
