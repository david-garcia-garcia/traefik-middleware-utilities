## prepare (2026-09-12)
phase: prepare
findings: none
fixed: none
skipped: Traefik per-request panic recovery not measured; AUTH/SELECT do-while-holding-token left out of scope; parser panic source (bug-02) not taken

## explore (2026-09-12)
phase: explore
findings: none
fixed: none
skipped: Traefik per-request panic recovery assumed not researched; Yaegi defer assumed from existing takeIdleConn; AUTH/SELECT handshake panic noted as debt; bug-02 parser cap not taken

## propose (2026-09-12)
phase: propose
findings: none
fixed: none
skipped: none; FindSpecHost folded into std_go_simpleredis_tcp-session

## implement (2026-09-12)
phase: implement
findings: none
fixed: doAndRelease returns the in-use-turn on panic; TestPanicDuringDoReturnsInUseTurn
skipped: handshake AUTH/SELECT panic still debt; parser cap (bug-02) not taken
