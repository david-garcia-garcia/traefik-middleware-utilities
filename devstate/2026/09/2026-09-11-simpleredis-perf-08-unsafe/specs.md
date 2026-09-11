# Specs
change: simpleredis-no-unsafe-zero-copy

verdicts:
  - { deltaId: session-copy-conversions, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands] }
  - { deltaId: yaegi-unsafe-guards, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }

- fold std_go_simpleredis_tcp-session — session source keeps `[]byte`/`string`; MUST NOT add unsafe zero-copy helpers
- fold std_go_simpleredis_resp-commands — asserting Yaegi conversion matrix, named copy-vs-unsafe benches, compiled import/manifest scan
