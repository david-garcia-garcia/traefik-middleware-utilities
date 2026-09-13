# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — Sleep/Wake panic endings still unmap before Close; spec Sleep-panic has no order vs EnforceCloseBeforeOpen
fixed: bus, requirement, stub PR #78
skipped: product apply; reproducer copy; nil-Done watcher leak; reclaim/BUGS.md

## explore (2026-09-13)
phase: explore
findings: both TestRepro_* fail on dest c230315 (incarnation 2 creates while Close of 1 blocked); dest has New(Config) not NewTable
fixed: explore.md with assumed ranks on helper name and dest constructor seam
skipped: product apply; usage rewrite (dest panic paths still unmap first)

## propose (2026-09-13)
phase: propose
findings: FindSpecHost fold std_go_reclaim_value-lifecycle and std_go_reclaim_context-lease; change reclaim-panic-enforce-close valid
fixed: proposal, delta specs, design, tasks
skipped: product apply

## implement (2026-09-13)
phase: implement
findings: TestRepro_* failed on dest then passed; full ./reclaim green
fixed: endBusyAfterPanic; closer slot on enforced path; usage packet
skipped: nil-Done watcher leak; reclaim/BUGS.md


