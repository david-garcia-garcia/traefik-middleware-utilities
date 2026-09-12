# Review

## prepare (2026-09-12)
phase: prepare
findings: none
fixed: none
skipped: product apply (prepare does not implement)

## explore (2026-09-12)
phase: explore
findings: dest readBulk accepts a full payload whose trailer is not CRLF
fixed: none
skipped: length cap (bug-02 out of scope); Accept-count export (infer two Accepts)

## propose (2026-09-12)
phase: propose
findings: fold into resp-decode and resp-commands
fixed: none
skipped: new spec leaf
