# Nitpicks

1. [hard] Name for the scope — `Test-Integration.ps1:22` — `Test-RedisHealth` names the host parameter `$RedisHost` but callers pass `"dragonfly"` for the Dragonfly wait; the reader must infer it is the `redis-cli -h` target, not the Redis service only
   → Rename to a backend-neutral role (`$BackendHost` / `$CliHost`) and keep `$ServiceName` for log text
   Status: done
   Argument: renamed parameter to $BackendHost (27d70e5).

2. [hard] Symmetry and consistency — `e2e/simpleredisprobe/plugin.go:62` — `ServeHTTP` keeps `got` for the Get reply while new verb blocks name the same role `incrValue`, `incrByValue`, and `values`
   → Use one stem per verb result (`getValue`, `incrValue`, `incrByValue`, `evalValues`) so sibling steps spell the same role
   Status: done
   Argument: Get result is getValue; Eval result is evalValues (27d70e5).

3. [hard] Linear coding — `simpleredis/simpleredis_test.go:81` — new `INCR` / `INCRBY` cases nest the success reply in `else` after `incrErr != nil` instead of guarding and leaving the reply on the left margin
   → Guard with `if incrErr != nil { write -ERR; break }` then `fmt.Fprintf` the `:` reply
   Status: done
   Argument: INCR/INCRBY guard then reply (27d70e5).

4. [hard] Name for the scope — `simpleredis/yaegi_test.go:140` — `IncrAndEval` stores the Incr reply in `n`; the next lines compare it to `1` and format it on failure
   → Rename to the role (`incrCount` / `afterIncr`)
   Status: done
   Argument: renamed Incr result to afterIncr (27d70e5).
