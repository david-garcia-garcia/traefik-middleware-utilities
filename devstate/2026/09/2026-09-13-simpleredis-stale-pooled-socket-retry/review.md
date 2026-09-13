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
