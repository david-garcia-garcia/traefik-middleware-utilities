# Explore
IssueKey: 2026-09-12-simpleredis-eval-caller-digest
Verdict: in progress

## Concepts

- **Eval** — dest `SimpleRedis.Eval(script, keys, args)` (`simpleredis/commands_eval.go:19-27`) hashes via unexported `scriptSHA1Hex` every call, sends `EVALSHA`, then `EVAL` on `NOSCRIPT`. Measured: `go test -short -count=1 -run 'TestEvalLaterSendsEvalShaNotBody|TestEvalArgvAndIntegerReply|TestEvalEmptyKeys' ./simpleredis/` passed. The ask is to stop hashing inside Eval and require the caller digest; keep the body for the existing fallback.
- **scriptSHA1Hex** — Redis `sha1hex` of the script bytes (SHA-1 lowercase 40-char hex). Unexported. Same-package uses: `commands_eval.go:21`, `commands_eval_test.go:43`, `fake_redis_test.go:190`. Research: `knowledge/research/ext_redis_evalsha/`.
- **Reuse callers** — production scripts are package consts. Dest hashes them on every Eval: `windowcounter/limiter.go:372` `flushScript`, `tokenbucket/redis.go:57` `allowScript`, `simpleredis/commands_msetex.go:64` `msetexFallbackScript`, `e2e/simpleredisprobe/plugin.go` (five Eval sites; `kongScriptDigest` already duplicates SHA-1 for the Pester header).
- **Spec host** — `openspec/specs/std_go_simpleredis_resp-commands` currently forbids a caller digest and requires hash-each-call. This change updates that requirement. Do not rewrite `std_go_tokenbucket_lua-eval` (“v1 MUST NOT use EVALSHA” stays a caller-surface rule: they still call `Eval`).
- **Usage** — `knowledge/devdocs/std_go_simpleredis.md` still describes dest Eval. Leave it until implement/devdocsimpact. No new research folder: EVALSHA digest and NOSCRIPT are already sourced.

```
Caller init: digest = ScriptSHA1Hex(script)   // once, next to the script const
        │
        ▼
Eval(script, digest, keys, args)
        │
        ▼
  EVALSHA digest numkeys keys args
        │
        ├─ NOSCRIPT ──► EVAL body numkeys keys args
        └─ result ──► caller [][]byte
```

## Decisions

- Signature becomes `Eval(script, digest, keys, args)`. Script stays first; keys/args stay last (Redis order). Digest is required. No optional overload, no `EvalSha`, no `SCRIPT LOAD`.
- Export `ScriptSHA1Hex` (Go export of dest `scriptSHA1Hex`). Same SHA-1 lowercase hex. Delete the unexported name; same-package tests and the fake call the export.
- Eval does not hash and does not check digest vs body (requirement Out of scope). EVALSHA uses the caller digest; `NOSCRIPT` still falls back once to `EVAL` of the body. A wrong digest repeats that fallback every call; document in usage, do not fail in Eval.
- Reuse callers hash once next to the script const (package `var` at init, or `New` for the probe). One-shot tests may call `ScriptSHA1Hex(script)` at the Eval site.
- Spec: update `std_go_simpleredis_resp-commands` Eval requirement. Do not rewrite `std_go_tokenbucket_lua-eval`.
- Probe: drop the local `kongScriptDigest` SHA-1 copy; `X-SimpleRedis-EvalDigest` uses `ScriptSHA1Hex`.
- Yaegi `clientprobe` must call `ScriptSHA1Hex` (interpreted, `stdlib.Symbols`, `useunsafe` false).
- Wire sequence, reply shape, no client digest table: unchanged.

## Open questions

- Q: Parameter order of the new digest argument on `Eval`?
  Rank: bounded asked — Desired 1 requires the SHA-1 hex; 44 existing `SimpleRedis.Eval` call sites migrate here (8 production in 4 files, 36 tests; searched `*.go` for `.Eval(`, excluded Yaegi `interpreter.Eval`)
  Decision: assumed — `Eval(script string, digest string, keys []string, args []string)`. Script stays first (dest first arg); keys/args stay last; digest sits next to the body it names.
  By: explore

- Q: Exported Go name for dest `scriptSHA1Hex`?
  Rank: additive asked — Desired 2 names exporting that hash; new public identifier
  Decision: assumed — `ScriptSHA1Hex`. That is the language-legal export of dest `scriptSHA1Hex`; Redis `sha1hex` of a script. No unexported alias.
  By: explore

- Q: Whether a caller-supplied digest that does not match the body should fail, or keep today’s NOSCRIPT then EVAL(body) path?
  Rank: additive asked — Unknowns names this; Out of scope forbids verifying digest vs body
  Decision: assumed — do not verify. Send the caller digest on EVALSHA; on NOSCRIPT send EVAL of the body (engine stores the body’s real SHA-1). Wrong digest pays EVAL every call; usage gotcha, not an Eval error.
  By: explore

- Q: Where do reuse callers store the digest?
  Rank: bounded asked — Desired 3 names hash at component init for reuse callers; 4 production owners (windowcounter, tokenbucket, msetex fallback, probe) enumerated above
  Decision: assumed — package-level digest next to each reused script const (`flushScript`, `allowScript`, `msetexFallbackScript`, probe script consts). Probe `New` may hash into the middleware if a field is clearer than a package var. One-shot tests hash at the call.
  By: explore

- Q: Should `std_go_tokenbucket_lua-eval` drop “v1 MUST NOT use EVALSHA”?
  Rank: additive incidental — no Desired line names that rewrite; Tensions says this ticket does not ask tokenbucket to stop using Eval
  Decision: assumed — leave that sentence. Tokenbucket still admits via `simpleredis.Eval`, not a new `EvalSha`. Wire EVALSHA stays SimpleRedis’s job.
  By: explore
