## Why

Dest has no Redis client. Middleware authors cannot import a shared SimpleRedis from this module; README still lists Redis as Planned. Crowdsec-bouncer already owns a stdlib RESP client at `pkg/simpleredis` — copy it here so other plugins reuse it under Yaegi instead of inventing another client.

## What Changes

- Add package `simpleredis/` (`package simpleredis`) copied from crowdsec-bouncer `pkg/simpleredis` @ `6548da47` (API, existing fake-TCP tests, Apache-2.0 package LICENSE). Import last segment stays `simpleredis`; do not rename to `redis`.
- Add `simpleredis/yaegi_test.go` mirroring `reclaim/yaegi_test.go` (GOPATH, stdlib only, `useunsafe` false).
- Add nested host plugin `e2e/simpleredisprobe` (module `github.com/david-garcia-garcia/simpleredisprobe`) that Inits in `New` and SET+GETs in `ServeHTTP`.
- Extend the existing compose project `reclaim-e2e`: `redis:7-alpine` at `redis:6379` (no password), second local plugin, a whoami that is not a/b on `/redis`. Keep reclaim `/a` `/b` and ports 8000/8080.
- One `Test-Integration.ps1` and one Pester file; add Redis + `/redis` waits. Redis Describe must not stop `whoami-a`/`whoami-b`.
- README Libraries title SimpleRedis (Current); Layout `simpleredis/`. No `go.mod` Redis require. No root LICENSE. Do not change reclaim semantics.

## Capabilities

### New Capabilities

- `std_go_simpleredis_tcp-session`: Stdlib TCP session — `Init` stores host/pass/database and does not dial; first command dials; AUTH/SELECT per dial; idle pool; `Close` drains and blocks redial; Traefik local-plugin load (`New` Inits only).
- `std_go_simpleredis_resp-commands`: RESP commands GET/MGET/SET/DEL and the exported error strings (`redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`). Interpreter tests must not start Traefik.

### Modified Capabilities

- None. Dest `std_go_reclaim_*` requirements stay as they are.

## Impact

- New `simpleredis/` (stdlib only) plus package-local `LICENSE`.
- New `e2e/simpleredisprobe/`, compose Redis + probe + `/redis` whoami, Pester Describe, CI failure logs for redis and the new whoami.
- README Libraries/Layout/Tests. No new `go.mod` require.
- Consumers later import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`. Crowdsec cache/LAPI stay out of scope.
- Reclaim package, reclaim e2e routes, and reclaim Pester semantics unchanged.
