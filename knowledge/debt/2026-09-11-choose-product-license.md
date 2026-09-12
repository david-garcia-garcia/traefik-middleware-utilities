# Choose a product LICENSE for dest

IssueKey: 2026-09-11-simpleredis
Size: large
Action: note

## Why this follow-up
Dest had no root `LICENSE`. SimpleRedis copied Apache-2.0 into `simpleredis/LICENSE`. Reclaim and the rest of the module still had no product license. The Apache-2.0 file now lives at repo-root `LICENSE` (Containous SAS / Traefik Labs appendix copyrights). That placement is easier to find; it does not by itself name a license for dest-original files such as `reclaim/`.

## Why it was not taken
Moving the Apache-2.0 file to the root does not name a license for dest-original `reclaim/` (and e2e). Legal/policy for those files is still a product choice.

## Risks
Downstream consumers treat the module as unlicensed except the copied client. Attribution for dest-original files stays unspecified.

## Context
This change originally shipped `simpleredis/LICENSE` (Apache-2.0 + Containous SAS / Traefik Labs copyrights) and a README note. That file is now repo-root `LICENSE`.
