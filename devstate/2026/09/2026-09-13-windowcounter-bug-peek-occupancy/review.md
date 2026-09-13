# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified — dest Peek is occupancy; godoc/spec/parent repro talk next-hit
fixed: bus, requirement, stub PR #64
skipped: product apply; Peek compare change

## explore (2026-09-13)
phase: explore
findings: occupancy reproduced (Peek true/est=3, Take false/est=4); godoc/spec still next-hit
fixed: explore.md with Rank/Decision; keep Peek compare
skipped: product apply

## propose (2026-09-13)
phase: propose
findings: fold std_go_windowcounter_sliding-take occupancy; tests first then docs
fixed: change windowcounter-peek-occupancy (proposal, spec delta, design, tasks)
skipped: product apply
