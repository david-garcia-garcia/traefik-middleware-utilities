# Delivery

## Motivation

Traefik operators wiring Redis rate limiting through `simpleredis.Config.Pass` sometimes see persistent WRONGPASS even when the password looks correct. Reports on https://github.com/traefik/traefik/issues/12078 trace that failure to Kubernetes Secret values that end with a literal newline: base64 `Cg==` is `\n`, not an empty suffix, so the engine receives a different password than the operator typed.

In this client, `New` already copies `cfg.Pass` without trimming and `authArgs` sends `[]byte(sr.pass)` on AUTH. That means `Pass` `"secret"` can succeed while `"secret\n"` is rejected as `redis:noauth`, matching the upstream symptom. The gap on master was proof and contract wording: there was no live test for the trailing-newline case, and the GoDoc on `Config.Pass` did not state byte-for-byte send or warn about Secret or file newlines. Without that, maintainers could add a silent trim “fix,” and operators would lack an in-repo anchor when debugging the same Secret mistake.

Leaving the gap alone does not change runtime behavior today, but it weakens the guardrail against credential transforms and leaves the traefik#12078 scenario undocumented beside the existing WRONGPASS live coverage (`"wrong-password"` only).

Priority: P3 — spec, docs, tests, or internal clarity with no new user or operator harm from this branch itself.

## Implementation

The binding keeps AUTH behavior unchanged and closes the gap on the `Config.Pass` documentation seam plus live proof on the requirepass engines.

The GoDoc on `Config.Pass` in `simpleredis/config.go` now states that AUTH receives the field byte for byte with no trimming, that a trailing newline from a Secret or file makes AUTH fail, and cites https://github.com/traefik/traefik/issues/12078. No edits to `New`, `authArgs`, `applyDefaults`, or `Config.User`.

A self-contained live file `simpleredis/traefik_issue12078_e2e_test.go` adds `TestTraefik12078_PasswordWithTrailingNewline`, copied from the reference repro. It reuses the package harness `runForEachLiveEngine` with `SIMPLEREDIS_LIVE_REDIS_AUTH` and `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` and `newTestRedis`. For each engine: control `Pass` `"secret"` must authenticate (Get succeeds or returns `redis:miss`); `Pass` `"secret\n"` must fail with `redis:noauth`. The test skips under `-short` or when those env vars are unset, same as other live AUTH tests in the package.

OpenSpec change metadata (`skip_specs: true`) records that the existing live AUTH and WRONGPASS catalog specs already cover the error mapping and this ticket adds no catalog delta.

## What this changes

**Operators.** When wiring `simpleredis.Config.Pass` from a Secret or file, ensure the value has no accidental trailing newline; the client sends the string exactly as stored and AUTH fails if an extra `\n` is present (see https://github.com/traefik/traefik/issues/12078).

**Admin users.** None.

**Developers.** `Config.Pass` GoDoc documents byte-for-byte AUTH, no trimming, and the newline failure mode with an upstream cite; `TestTraefik12078_PasswordWithTrailingNewline` locks that contract on the requirepass live engines without changing session code.

**End users.** None.