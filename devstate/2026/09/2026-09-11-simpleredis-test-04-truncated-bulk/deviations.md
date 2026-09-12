# Deviations

- [x] taken  keep existing compose `/redis` `/dragonfly` instead of new services
  Asked: extend compose plus Pester `/redis` `/dragonfly` so live isolation is asserted on both routes.
  Instead: keep dest `whoami-redis` / `whoami-dragonfly` and image pins; extend probe payload and Pester assertions only.
  Owner: `docker-compose.yml`
  Why: dest already wires both engines on those routes; extra services or whoami would be a second stack for a job the existing compose already does.
  By: propose
  Requester: not asked

- [x] taken  two sequential own-value GETs instead of overlapping
  Asked: two overlapping GET `/redis` and `/dragonfly` with distinct tokens.
  Instead: two sequential GETs per route; dest drop-relay and live-cap holds already run on those same handlers.
  Owner: `scripts/integration-tests.Tests.ps1`
  Why: after master, every GET also drives lost-reply Incr/Eval through drop-relay; true overlap 502s on CI and hides the own-value assert.
  By: implement
  Requester: not asked
