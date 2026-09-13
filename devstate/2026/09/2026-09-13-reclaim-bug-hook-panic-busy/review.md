## prepare (2026-09-13)
phase: prepare
findings: none
fixed: none
skipped: other two reclaim bugs (canceled-ctx Open returns closed value; unmap-before-Close overlap)

## explore (2026-09-13)
phase: explore
findings: 2a–2d reproduced on master (hang / leftover slotBusy / AfterFunc child exit status 2)
fixed: none
skipped: other two reclaim bugs

## implement (2026-09-13)
phase: implement
findings: none
fixed: recover at put/drop/reclaimLocked/dispose/Reset; tests 2a–2d fail-then-pass
skipped: other two reclaim bugs
