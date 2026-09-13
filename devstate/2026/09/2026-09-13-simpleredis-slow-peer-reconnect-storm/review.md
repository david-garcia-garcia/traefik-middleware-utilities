## prepare (2026-09-13)

phase: prepare
findings: none
fixed: none
skipped: independent reproduction run; product code

## explore (2026-09-13)

phase: explore
findings: 15-dial collapse reproduced; serial 9.9 dials/s at default 100ms; concurrent peak server open 2x PoolSize; AUTH/SELECT tax measured
fixed: none (docs-only recommendation)
skipped: circuit breaker; default IOTimeout change; default-suite churn bound

## propose (2026-09-13)

phase: propose
findings: docs-only change simpleredis-timeout-connect-per-command; spec ADDED per-command timeout rule; usage gotcha drafted
fixed: none (runtime)
skipped: implement gated for the human; no breaker
