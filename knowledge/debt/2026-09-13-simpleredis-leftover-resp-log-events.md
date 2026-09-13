# Add leftover-RESP log events after PR #69

IssueKey: 2026-09-13-simpleredis-structured-logging
Size: large
Action: note

## Why this follow-up
Owner inventory includes `simpleredis_socket_poisoned` (leftover RESP after a complete reply) and `simpleredis_auth_leftover` (pre-write reader not empty on the handshake path). Dest `origin/master` (`c14cbec`) has no `Buffered()` check in `simpleredis/resp.go` `do` or in `dial`; those sites land with PR #69.

## Why it was not taken
PR #69 is still OPEN and not in dest. The ticket forbids inventing leftover detection in this change. Unattended take is only small rows on files this run created.

## Risks
Those two Warn events stay silent after this PR until a later change wires them at #69's detection sites. Operators will not see leftover-RESP poison or AUTH leftover in logs even when those conditions happen.

## Context
Current: dest `simpleredis/resp.go` `do` returns `reusable true` after a well-formed `readReply` with no leftover check. `dial` ignores `reusable`.
Proposed: after #69 merges, emit `MsgSocketPoisoned` / `MsgAuthLeftover` at those sites with `buffered` and `host`, never keys or values.
