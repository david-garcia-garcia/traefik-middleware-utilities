---
url: http://oldblog.antirez.com/post/scripting-branch-released.html
title: Scripting branch released
fetched: 2026-09-11
authority: official
---

Return type mapping (original EVAL release):
- Lua number → Redis integer reply
- Lua string → Redis bulk reply
- Lua array → Redis multi bulk reply
- Lua table with err field → Redis error reply
- Lua table with ok field → Redis status reply

Example script error:
EVAL "return {ok=os.currentdir()}" 0
→ (error) ERR Error running script (call to f_3cba57088cac1f60388c6085385a464eef74f42c): [string "func definition"]:2: attempt to call field 'currentdir' (a nil value)

Redis embeds Lua 5.1 (_VERSION returns "Lua 5.1").
