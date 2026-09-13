## prepare (2026-09-13)
phase: prepare
findings: none
fixed: none
skipped: product apply (prepare only). Epoch vs cheap reuse-flag left for explore/propose. Tagged PRODUCTION-BUGS reproductions stay untracked on dest.

## explore (2026-09-13)
phase: explore
findings: reproduced 4 sequential redis:unreachable at PoolSize 8 / MaxRetries 1
fixed: none
skipped: epoch / idle-list wipe. Cheap skipIdle plus one unused-socket force-dial proceeds.

## propose (2026-09-13)
phase: propose
findings: fold std_go_simpleredis_tcp-session; change simpleredis-stale-pooled-socket-retry
fixed: none
skipped: epoch. Apply not started.

## implement (2026-09-13)
phase: implement
findings: skip-idle after unused-socket unreachable; no free extra send
fixed: sequential Gets after full idle drop at default MaxRetries
skipped: MaxRetries -1 extra send (lost-reply collision); epoch; idle wipe

## codereview (2026-09-13)
phase: codereview
findings: Standards 2 hard (stale name, serve comment); Dead 1 hard (leftover borrow)
fixed: renamed fake to peerDropAllFake; serve job comment; deleted borrow; tests call borrowSocket(ctx, false)
skipped: none

## devdocsimpact (2026-09-13)
phase: devdocsimpact
findings: stale-usage on std_go_simpleredis idle-socket gotcha
fixed: skip-idle sequential recovery on the SimpleRedis usage packet
skipped: none

## archive (2026-09-13)
phase: archive
findings: none
fixed: folded Peer-closed idle vintage into std_go_simpleredis_tcp-session; moved change to archive
skipped: none
