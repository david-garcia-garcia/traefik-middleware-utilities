# Test coverage

1. [judgement] Assertion does not prove the job — `simpleredis/simpleredis.go:166` `Eval` hashes each call, sends `EVALSHA`, and on `strings.HasPrefix(err.Error(), noScriptPrefix)` falls back once to `EVAL` — `simpleredis/yaegi_test.go:TestYaegi_EvalNoScriptFallback` / `clientprobe.EvalNoScript` only assert interpreted return `"ok"` (result bytes and no error string); reverting `Eval` to always-`EVAL` would stay green while the EVALSHA-first wire path is untested under Yaegi (compiled fake tests `TestEvalArgvAndIntegerReply`, `TestEvalLaterSendsEvalShaNotBody`, `TestEvalEmptyKeysSendsNumkeysZero` would fail on that revert)
   → Mirror fake `evalCommandCounts` or assert first command is `EVALSHA` then one `EVAL` in the Yaegi probe, or accept compiled argv tests as sufficient and drop the redundant Yaegi case
   Status: skipped
   Argument: judgement; compiled fake argv tests prove EVALSHA-first; Yaegi still proves NOSCRIPT fallback succeeds as required.
