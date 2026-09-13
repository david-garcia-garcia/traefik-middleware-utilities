# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — spec still fails construction only when ttl < 1s; ticket requires whole seconds
fixed: bus, requirement, stub PR #57
skipped: product apply; other tokenbucket bugs

## explore (2026-09-13)
phase: explore
findings: dest New(1500ms) accepted; Redis ARGV ttl 1; Memory still live at +1200ms
fixed: explore.md proceed policies (errTTL text, ttl_truncation_test.go, both constructors)
skipped: product apply; PEXPIRE; Lua rewrite

## propose (2026-09-13)
phase: propose
findings: fold std_go_tokenbucket_allow and std_go_tokenbucket_lua-eval
fixed: change tokenbucket-ttl-whole-seconds (proposal, deltas, design, tasks)
skipped: product apply

## implement (2026-09-13)
phase: implement
findings: dest FAIL then whole-second validateClock PASS
fixed: errTTL whole seconds; ttl_truncation_test.go reshaped; usage gotcha
skipped: PEXPIRE; Lua rewrite

## codereview (2026-09-13)
phase: codereview
findings: seven axes none
fixed: none
skipped: none

## devdocsimpact (2026-09-13)
phase: devdocsimpact
findings: none — usage packet already has whole-second ttl gotcha
fixed: none
skipped: none

## archive (2026-09-13)
phase: archive
findings: fold allow and lua-eval
fixed: live specs synced; change moved to archive/2026-09-13-tokenbucket-ttl-whole-seconds
skipped: none


