# Standards

1. [judgement] Duplicated Code — `e2e/simpleredisprobe/plugin.go:28` — `kongScriptDigest` repeats Redis sha1hex already owned by `scriptSHA1Hex` in `simpleredis/simpleredis.go:190`
   → Call one sha1hex owner, or leave the probe copy because the plugin cannot import the unexported helper and the spec forbids exporting EvalSha
   Status: skipped
   Argument: judgement; probe cannot import unexported scriptSHA1Hex and exporting a digest helper is a new public surface the spec forbids pairing with EvalSha.
