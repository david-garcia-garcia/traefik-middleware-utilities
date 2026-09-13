# Test coverage

1. [hard] Critical path untested — `simpleredis/resp.go:34` — leftover before write now fail-closed (`errIssue`, no write) so AUTH leftover is not parsed as SELECT; `TestDesyncedSocketDoesNotServePreviousReplies`, `TestStrayExtraReplyIsNotPooled`, and `TestAuthAndSelectOncePerDial` never leave unread bytes on the same socket for a second `do` (`(none)`)
   → Assert AUTH leftover then SELECT is not written, handshake fails with redis:issue, socket is not pooled
   Status: done
   Argument: added `TestAuthLeftoverIsNotParsedAsSelect` (AUTH leftover, SELECT not written, redis:issue, idle empty).
