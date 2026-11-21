# Devdocs impact

Units: `std_go_simpleredis` (usage), `std_go_simpleredis_resp-decode` (no change — maxBulkLength stays a parse cap).

Findings:
- `knowledge/devdocs/std_go_simpleredis.md` usage retry bullet still said one-shot `SetDeadline`. Updated this apply to stall refresh plus overall-budget close.
- Language has no new term: the Config field stays `IOTimeout`; meaning moved in usage.
- RESP-decode packet already says keep `maxBulkLength` off Config. Matches the deviation.
