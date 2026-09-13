# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — spec only names rate <= 0; ticket also rejects NaN and Inf
fixed: bus, requirement, stub PR #52
skipped: product apply; other tokenbucket bugs

## explore (2026-09-13)
phase: explore
findings: dest accepts NaN and +Inf at New; Allow fail-open; -Inf already errRate
fixed: none (think-only)
skipped: product apply

## propose (2026-09-13)
phase: propose
findings: fold std_go_tokenbucket_allow; tests-first tasks
fixed: OpenSpec change tokenbucket-reject-nonfinite-rate
skipped: product apply

