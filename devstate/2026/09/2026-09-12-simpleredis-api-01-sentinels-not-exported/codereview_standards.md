# Standards

1. [hard] Leave a trail — `simpleredis/yaegi_test.go:68` — job comment still names MSetEX Lua fallback after the function became MatchSentinels
   `// TestYaegi_MSetEXLua proves interpreted MSetEX falls back to EVAL and a second call skips MSETEX.` then `func TestYaegi_MatchSentinels`
   → Comment that interpreted `errors.Is` and `IsMiss` match a wrapped `ErrMiss`
   Status: done
   Argument: comment now names MatchSentinels job (errors.Is + IsMiss). SHA 7e2b88e.
