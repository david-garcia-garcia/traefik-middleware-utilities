# Nitpicks

1. [hard] Name for the scope — `simpleredis/commands_deadline_test.go:26` — `startStallRedis` names the accepted socket `c`:
   `go func(c net.Conn) { defer c.Close(); _, _ = io.Copy(io.Discard, c) }(conn)`
   → `go func(conn net.Conn) { defer conn.Close(); _, _ = io.Copy(io.Discard, conn) }(conn)`
   Status: done
   Argument: accepted socket parameter is `conn`.
