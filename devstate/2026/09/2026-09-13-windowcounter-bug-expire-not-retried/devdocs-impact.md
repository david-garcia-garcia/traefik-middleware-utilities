# Devdocs impact
change: windowcounter-exact-expire-if-no-ttl

## Units
- Window counter — subsystem — `windowcounter/` / `std_go_windowcounter.md`

## Findings
- [x] stale-usage  Window counter — `std_go_windowcounter.md` exact path was `INCR` + `EXPIRE` on first hit; produced to EVAL expire-if-no-TTL (`PTTL < 0`) plus Gotcha
