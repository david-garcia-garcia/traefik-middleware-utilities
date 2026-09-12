# Deviations

- [x] taken  hash every Eval instead of a client digest cache and mutex
  Asked: cache SHA-1 of each distinct script on the client (`crypto/sha1`, mutex-guarded map); send EVALSHA; on NOSCRIPT fall back once to EVAL then keep using the digest.
  Instead: `Eval` hashes the body each call with `scriptSHA1Hex` and sends EVALSHA; NOSCRIPT still falls back once to EVAL (the only send of the body). No `digestMu`, `digests`, or `cachedScriptDigest`.
  Owner: `simpleredis/simpleredis.go` `Eval`
  Why: honouring the cache adds a mutex and a `map[string]string` of full script bodies on the Traefik hot path; SHA-1 of a limiter script (~470 B) is cheaper than that lock, and Go maps are not concurrent.
  By: implement
  Requester: confirmed
