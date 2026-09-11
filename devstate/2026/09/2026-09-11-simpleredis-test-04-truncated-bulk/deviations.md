# Deviations

- [x] taken  keep existing compose `/redis` `/dragonfly` instead of new services
  Asked: extend compose plus Pester `/redis` `/dragonfly` so live isolation is asserted on both routes.
  Instead: keep dest `whoami-redis` / `whoami-dragonfly` and image pins; extend probe payload and Pester assertions only.
  Owner: `docker-compose.yml`
  Why: dest already wires both engines on those routes; extra services or whoami would be a second stack for a job the existing compose already does.
  By: propose
  Requester: not asked
