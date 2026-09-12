# Test coverage

1. [hard] Assertion does not prove the job — `simpleredis/commands_eval.go:26` — `Eval` sends the caller `digest` on EVALSHA and MUST NOT hash; `TestEvalUsesCallerDigest` (`simpleredis/commands_eval_test.go:131`) passes `ScriptSHA1Hex(script)` so the EVALSHA argv still matches if Eval hashed the body again
   → Assert EVALSHA argv is a caller digest that is not `ScriptSHA1Hex` of that script
   Status: done
   Argument: TestEvalUsesCallerDigest now preloads a digest that is not ScriptSHA1Hex(script) and asserts EVALSHA argv is that caller string.
