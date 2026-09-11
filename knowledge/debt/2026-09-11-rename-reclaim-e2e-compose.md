# Rename compose project `reclaim-e2e` to a harness name

IssueKey: 2026-09-11-simpleredis
Size: large
Action: note

## Why this follow-up
`docker-compose.yml` `name: reclaim-e2e` and container `reclaim-e2e-traefik` name a reclaim-only stack. After SimpleRedis lands, the same project also loads `simpleredisprobe` and a Redis service.

## Why it was not taken
Pester hardcodes `reclaim-e2e-traefik`; Traefik Docker constraints use `com.docker.compose.project=reclaim-e2e`; CI dumps that container. Unattended take is only small rows on files this run created. Existing reclaim e2e must keep working.

## Risks
Later libraries keep folding into a reclaim-named harness. Operators reading compose project name miss that Redis is in the same stack.

## Context
Current: `docker-compose.yml` `name: reclaim-e2e`, `container_name: reclaim-e2e-traefik`, `scripts/integration-tests.Tests.ps1` `docker logs reclaim-e2e-traefik`.
Proposed: a harness name that does not name one library (only with a measured rename of those three plus CI).
