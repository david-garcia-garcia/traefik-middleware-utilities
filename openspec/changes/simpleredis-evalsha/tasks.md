## 1. Client EVALSHA

- [ ] 1.1 Add a mutex-guarded script-to-SHA-1 hex map on `SimpleRedis` (own mutex, not pool `mu`). Digest is `crypto/sha1` of the script bytes, lowercase hex. Do not hold pool `mu` across `exec`. Do not `SCRIPT LOAD` at `Init`.
- [ ] 1.2 Change `Eval` to send `EVALSHA` digest `numkeys` keys args. On `strings.HasPrefix(err.Error(), "NOSCRIPT")` send `EVAL` once with the same body/keys/args and keep the digest. Do not return that NOSCRIPT. Do not map NOSCRIPT in `replyError`. Do not special-case NOSCRIPT in `exec`. Public signature stays `Eval(script, keys, args)`.
- [ ] 1.3 Extend `startFakeRedis`: handle `EVALSHA`; first miss `-NOSCRIPT No matching script. Please use EVAL.`; `EVAL` loads that digest; later EVALSHA runs the same Kong path as EVAL. Record last argv (digest not body). Unknown-command `+OK` MUST NOT remain the EVALSHA path.

## 2. Compiled and Yaegi tests

- [ ] 2.1 Compiled: first Eval falls back EVAL after NOSCRIPT; second Eval sends EVALSHA + digest not the body; two scripts get two digests; empty keys still `numkeys` `0`; integer/mixed/nested reply tests still pass. Existing Get/Set/Del/MGet/Incr still pass.
- [ ] 2.2 Yaegi: extend `clientprobe` so interpreted `Eval` hits the NOSCRIPT fallback against the compiled fake (`stdlib.Symbols`, `useunsafe` false, no Traefik). Keep `TestYaegi_IncrAndEval`.
- [ ] 2.3 Run `go test ./simpleredis/...` until compiled and Yaegi tests pass.

## 3. Live Redis and Dragonfly

- [ ] 3.1 Probe: keep Kong KEYS snippet (no `table.maxn`). Set `X-SimpleRedis-EvalDigest` to SHA-1 of that const. Call Eval a second time; set `X-SimpleRedis-EvalAgain`. Keep existing verb headers. No new compose services or routes.
- [ ] 3.2 Pester `/redis`: `docker compose exec -T redis redis-cli SCRIPT FLUSH` then `SCRIPT EXISTS` of that digest (`0`), GET succeeds (`Eval` `3`, `EvalAgain` `3`, digest header matches), `EXISTS` `1`, GET again `3`. Same sequence on `/dragonfly` via `redis-cli -h dragonfly`. Do not stop whoami-a/b.
- [ ] 3.3 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass and reclaim stays green.

## 4. Specs

- [ ] 4.1 Confirm the change delta `std_go_simpleredis_resp-commands` matches the landed API (public Eval unchanged; EVALSHA-inside-Eval; live both engines).
- [ ] 4.2 Run `openspec validate --change simpleredis-evalsha --strict`
