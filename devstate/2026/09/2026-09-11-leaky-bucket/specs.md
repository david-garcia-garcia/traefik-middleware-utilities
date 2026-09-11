# Specs
change: add-leakybucket

FindSpecHost:

```
verdicts:
  - { deltaId: pour, fold|new: new, spec-id: std_go_leakybucket_pour, confidence: high, candidates: [std_go_tokenbucket_allow, std_go_windowcounter_sliding-take] }
  - { deltaId: sync-flush, fold|new: new, spec-id: std_go_leakybucket_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_tokenbucket_lua-eval] }
```

- added std_go_leakybucket_pour
- added std_go_leakybucket_sync-flush
