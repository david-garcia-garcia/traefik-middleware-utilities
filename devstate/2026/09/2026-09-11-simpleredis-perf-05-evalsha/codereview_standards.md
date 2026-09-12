# Standards

1. [judgement] Duplicated Code — `e2e/simpleredisprobe/plugin.go:27`, `scripts/integration-tests.Tests.ps1:22`, `simpleredis/simpleredis.go:172` — three copies of Redis sha1hex for the same Kong const (`kongScriptDigest`, `$script:KongEvalDigest`, `scriptSHA1Hex`); editing `kongIncrbyExpireatScript` can desync Pester EXISTS from the probe header
   → Single owner (e.g. small exported digest helper on `simpleredis` used by probe, tests derive expected hex from the const once)
   Status: skipped
   Argument: judgement; probe cannot import unexported scriptSHA1Hex and exporting a digest helper is a new public surface the spec forbids pairing with EvalSha.
