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

