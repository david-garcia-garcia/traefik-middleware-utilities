# Bring simpleredis into traefik-middleware-utilities as an importable package

Bring the simpleredis package from https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/tree/master/pkg/simpleredis into this project as a package so other projects can import it (`github.com/david-garcia-garcia/traefik-middleware-utilities`).

Requirements:
1. Copy the existing simpleredis sources and their existing test coverage from that repo.
2. Besides those tests, create Yaegi-specific tests (this repo’s libraries run under Traefik’s Yaegi interpreter; see README Yaegi rules and existing reclaim Yaegi tests on dest).
3. Create a host plugin that uses Redis so we can add wrap-up e2e tests with Pester (follow the existing reclaim e2e pattern: `e2e/`, docker-compose, `Test-Integration.ps1`, `scripts/integration-tests.Tests.ps1`).
4. Work is a new product library (README currently lists Redis connection as Planned; layout expects `redis/`).
