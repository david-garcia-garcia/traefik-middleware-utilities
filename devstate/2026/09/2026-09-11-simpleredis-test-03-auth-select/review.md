## prepare (2026-09-11)

phase: prepare
findings: none
fixed: none
skipped: none

## explore (2026-09-11)

phase: explore
findings: coverage gap reproduced (dial AUTH/SELECT error blocks count 0); Redis 7.4 nopass AUTH is not redis:noauth; Dragonfly nopass AUTH is OK; both engines WRONGPASS with requirepass and SELECT 99 out of range
fixed: none
skipped: none

## propose (2026-09-11)

phase: propose
findings: fold std_go_simpleredis_tcp-session and std_go_simpleredis_resp-commands; OpenSpec change simpleredis-test-03-auth-select valid
fixed: none
skipped: none

## implement (2026-09-11)

phase: implement
findings: fake AUTH/SELECT failure tests; live skip-if-unset Redis and Dragonfly; CI passworded siblings; Pester 502 routes
fixed: handshake coverage on dial AUTH/SELECT error blocks; probe Password/Database
skipped: none

## codereview (2026-09-11)

phase: codereview
findings: none
fixed: none
skipped: none

