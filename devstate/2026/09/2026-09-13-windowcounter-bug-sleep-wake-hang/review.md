# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — spec Wake SHALL start when sync_rate > 0 has no Sleep-in-progress exception; dest stopFlushAndWait clears stop before wg.Wait
fixed: bus, requirement, stub PR #61
skipped: product apply; other windowcounter bugs

## explore (2026-09-13)
phase: explore
findings: dest race hung iteration 2 after 2s; Sleep-wins stopping flag; four assumed additive asked
fixed: explore.md
skipped: product apply

## propose (2026-09-13)
phase: propose
findings: fold std_go_windowcounter_sync-flush; change windowcounter-sleep-wins-wake valid
fixed: proposal, delta spec, design, tasks
skipped: limiter apply

## implement (2026-09-13)
phase: implement
findings: stopping flag; race test FAIL then pass; localTests passed
fixed: limiter.go Wake/stopFlushAndWait; TestRepro_SleepWakeRaceHangs; TestWake_StartsTickerAfterSleep; usage gotcha
skipped: none in scope

## codereview (2026-09-13)
phase: codereview
findings: coverage 1 hard (already-stopped Sleep then Wake); six axes none
fixed: TestWake_StartsTickerAfterSleep second Sleep
skipped: none

## impact (2026-09-13)
phase: devdocsimpact
findings: none — std_go_windowcounter Gotchas already name Sleep-wins
fixed: none
skipped: Language (no new term)

## archive (2026-09-13)
phase: archive
findings: fold std_go_windowcounter_sync-flush; moved archive/2026-09-13-windowcounter-sleep-wins-wake
fixed: live spec Sleep Wake Close; folder move
skipped: none

