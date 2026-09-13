## prepare (2026-09-13T06:12:12Z)
phase: prepare
findings: none
fixed: none
skipped: wait mapping, NaN rate, ttl truncation, last rewind, x/time/rate

## explore (2026-09-13T06:17:21Z)
phase: explore
findings: dest Memory epoch tokens=-1 last=0; huge burst 1.7e9 not burst-1; fake Redis missing hash same
fixed: none
skipped: wait mapping, NaN rate, ttl truncation, last rewind, x/time/rate

## propose (2026-09-13T06:21:19Z)
phase: propose
findings: none
fixed: none
skipped: wait mapping, NaN rate, ttl truncation, last rewind, x/time/rate

## implement (2026-09-13T06:25:12Z)
phase: implement
findings: FAIL then PASS TestRepro_NewKeyFillsToBurstAtEpoch (a432483 → 5ba03a1)
fixed: Lua empty hash and Memory new memEntry seed burst/now; fake missing hash matches
skipped: wait mapping, NaN rate, ttl truncation, last rewind, x/time/rate

## codereview (2026-09-13T06:30:04Z)
phase: codereview
findings: seven axes none
fixed: none
skipped: none

## devdocsimpact (2026-09-13T06:33:14Z)
phase: devdocsimpact
findings: none (usage gotcha already on std_go_tokenbucket.md)
fixed: none
skipped: none

## archive (2026-09-13T06:33:14Z)
phase: archive
findings: none
fixed: none
skipped: none
