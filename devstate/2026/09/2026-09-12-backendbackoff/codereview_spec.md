# Spec

1. [wrong] `backendbackoff/allow.go:57` — Report deletes the key and returns without recording when `expireAt` has passed; spec Requirement: Report applies saturating credit SHALL record an admitted attempt, and Requirement: Memory map bounds idle keys expires idle keys on a later Allow, not on Report
   Status: done
   Argument: Report no longer deletes on expire; expire stays Allow-only. Added TestReport_AfterTTLStillRecords.
