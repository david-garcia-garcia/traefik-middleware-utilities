# Specs
change: simpleredis-eval-fallback-hop-comments
- fold std_go_simpleredis_resp-commands (high) — Eval / MSetEX fallback hops each have a full command budget; comments + tests that pass on dest
- not fold std_go_simpleredis_tcp-session — requirement Out of scope; explore assumed
