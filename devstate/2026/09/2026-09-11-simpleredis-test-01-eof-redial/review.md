## prepare (2026-09-11)
phase: prepare
findings: qualified-with-gaps; live idle-close mechanism for Redis and Dragonfly unknown
fixed: n/a
skipped: n/a

## explore (2026-09-11)
phase: explore
findings: 6 open questions (4 assumed, 2 resolved, 0 blocked); live close is CLIENT KILL ADDR/ID on both engines
fixed: n/a
skipped: n/a

## propose (2026-09-11)
phase: propose
findings: change simpleredis-peer-close-eof-redial; fold std_go_simpleredis_tcp-session; 4 assumed rows remain
fixed: n/a
skipped: n/a

## implement (2026-09-11)
phase: implement
findings: peer-close fake, live CLIENT KILL on Redis and Dragonfly, recover=1 Pester; dest exec unchanged; CI 34651839729 succeeded
fixed: n/a
skipped: n/a

## codereview (2026-09-11T22:13:40.664Z)
phase: codereview
findings: Standards 1 hard done, Standards 2 judgement skipped; Nitpicks 1-2 hard done; Spec/Security/Performance/Dead/Coverage none
fixed: Leave a trail comments on `Stop-TraefikEngineClientForTest`; `killPooledIdleAddrOrIDForTest`; `peerCloseFake.connections` (`60346e9`)
skipped: extract overlapping Redis/Dragonfly Pester Its (judgement; engines stay visible as dest backends)
