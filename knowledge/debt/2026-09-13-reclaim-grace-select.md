# Reclaim grace waiter still uses interpreted `go` + `select`

IssueKey: 2026-09-13-windowcounter-bug-yaegi-flush-hang
Size: large
Action: note

## Why this follow-up
`reclaim/table.go` `waitGraceOrWake` (line 419 `go t.waitGraceOrWake`, line 427 `select` on `wait.C` and `woken`) is the same Yaegi-unsafe shape as the old `windowcounter` `flushLoop`: an interpreted goroutine selecting on a timer channel and a stop channel.

## Why it was not taken
Grace/wake/expire protocol is not a cheap timer swap. This change only converts the windowcounter flush path. Unattended take is only small rows on files this run created.

## Risks
Interpreted reclaim Sleep with positive grace could hang a Traefik reload the same way buffered `windowcounter` Sleep hung, if Yaegi misses the `woken` close.

## Context
Current: `reclaim/table.go` `waitGraceOrWake`
windowcounter fix: `time.AfterFunc` in `windowcounter/limiter.go`
