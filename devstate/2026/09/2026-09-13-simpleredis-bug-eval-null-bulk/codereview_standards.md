# Standards

1. [hard] Leave a trail — `simpleredis/resp.go:229` — `parseLen` still says an optional minus is accepted so `$-1 stays a miss` after top-level `$` now returns a nil slot with a nil error, not `redis:miss`
   → Rewrite that comment to the packet: optional minus is `$-1` nil slot, `*-1` negative array
   Status: done
   Argument: Rewrote parseLen comment so optional minus is $-1/`*-1` negative length, not miss (64dba6a).
