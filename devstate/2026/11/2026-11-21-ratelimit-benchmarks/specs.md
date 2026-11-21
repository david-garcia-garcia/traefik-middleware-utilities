# Specs
change: ratelimit-benchmarks-key-scaling
- modified std_go_tokenbucket_allow
  Verdict: fold (high) — one ADDED requirement: in-memory source cap 65536, expired sources removed before any live one, removed source restarts as a full bucket; victim (LRU) not promised. Candidates: std_go_tokenbucket_allow, std_go_tokenbucket_lua-eval.
- modified std_go_windowcounter_sync-flush
  Verdict: fold (high) — MODIFIED "Buffered mode shares one limit without last-write-wins" and "Flush EVAL integer parse keeps the cause"; ADDED "Buffered flush batches keys with a per-key outcome" (≤1024 keys per EVAL, per-key outcome, no re-send of applied keys). Candidates: std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take, std_go_simpleredis_resp-commands.
- skip benchmarks, fake Redis digest cache, README, devdocs — no live contract (test-only files and docs).
