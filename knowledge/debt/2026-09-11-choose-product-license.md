# Choose a product LICENSE for dest

IssueKey: 2026-09-11-simpleredis
Size: large
Action: note

## Why this follow-up
Dest has no root `LICENSE`. This change copies Apache-2.0 SimpleRedis into `simpleredis/` with a package-local LICENSE. Reclaim and the rest of the module still have no product license.

## Why it was not taken
A root LICENSE would relicense existing `reclaim/` (and e2e) without a criterion naming it. Legal/policy for the whole repo. Unattended take is only small rows on files this run created.

## Risks
Downstream consumers treat the module as unlicensed except the copied client. Attribution for dest-original files stays unspecified.

## Context
This change ships `simpleredis/LICENSE` (Apache-2.0 + Containous SAS / Traefik Labs copyrights) and a README note. It does not add a repo-root LICENSE.
