# Requirement

After #27 squash-merged as `1ed5640`, master code honours `MaxIdleConns` on unused sockets (idle-only close). The live spec `std_go_simpleredis_tcp-session` auto-merged two opposite close rules. `TestReleaseClosesWhenIdleFullAtLiveCap` still names a live-cap conjunct the code no longer checks.

Desired: rewrite the panic/unlock requirement so it keeps that invariant and restates close as idle-only, without repeating L56. Rename the test. No behaviour change.
