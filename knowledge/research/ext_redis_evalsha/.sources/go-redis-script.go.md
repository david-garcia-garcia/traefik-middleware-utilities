---
url: https://github.com/redis/go-redis/blob/v9.21.0/script.go
title: go-redis Script.Run EVALSHA then EVAL
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis/v9@v9.21.0:script.go
---

NewScript: crypto/sha1 of src, encoding/hex.EncodeToString (lowercase hex).

Run: EvalSha first; if errors.Is(err, ErrNoScript) then Eval with the body.

EvalSha (serverSHA mode) retries SCRIPT LOAD when HasErrorPrefix(err, "NOSCRIPT").
