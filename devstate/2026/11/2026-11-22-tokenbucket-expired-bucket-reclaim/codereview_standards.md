# Standards

1. [hard] Bound the ask — `knowledge/opendev.json:1` — the apply adds OpenDev workspace settings and `.cursorignore`; neither is expired-bucket reclaim
   Quote:
      ```
      /devstate/
      /openspec/changes/archive/
      ```
   Fix: Remove `knowledge/opendev.json` and `.cursorignore` from the branch (or land them in a separate change if the repo versions them)
   Status: done
   Argument: removed `knowledge/opendev.json` and `.cursorignore` from the branch.
