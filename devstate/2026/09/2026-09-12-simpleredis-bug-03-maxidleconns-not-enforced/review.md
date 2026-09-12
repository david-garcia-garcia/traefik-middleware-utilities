## prepare (2026-09-12)
phase: prepare
findings: none
fixed: none (no product apply)
skipped: clamp MaxIdleConns to PoolSize; idle reaper / other simpleredisfixes2 files

## explore (2026-09-12)
phase: explore
findings: DestBranch idle grows past MaxIdleConns (8/2 → idle 7); spec MUST NOT-close-under-PoolSize contradicts Config
fixed: none (no product apply)
skipped: clamp MaxIdleConns to PoolSize; idle reaper; external peak sampler
