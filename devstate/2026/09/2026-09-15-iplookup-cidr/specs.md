# Specs
change: iplookup-cidr-helper
verdicts:
  - { deltaId: family-match, fold: new, spec-id: std_go_iplookup_family-match, confidence: high, candidates: [std_go_backendbackoff_allow, std_go_reclaim_context-lease] }
  - { deltaId: cidr-store, fold: new, spec-id: std_go_iplookup_cidr-store, confidence: high, candidates: [std_go_backendbackoff_allow] }
  - { deltaId: match-label, fold: new, spec-id: std_go_iplookup_match-label, confidence: high, candidates: [std_go_backendbackoff_allow] }
- added std_go_iplookup_family-match
- added std_go_iplookup_cidr-store
- added std_go_iplookup_match-label
