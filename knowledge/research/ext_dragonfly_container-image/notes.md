# Dragonfly official container image

Facts for running Dragonfly beside `redis:7-alpine` in tiny CI e2e.

## Official image name and pin

| Item | Value |
|------|-------|
| Registry image | `docker.dragonflydb.io/dragonflydb/dragonfly` |
| **Pinned tag (this ticket)** | **`v1.40.2`** |
| Full reference | `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` |

Official install docs use the `docker.dragonflydb.io` registry, not bare `dragonflydb/dragonfly` on Docker Hub. ([dragonflydb.io Install with Docker](https://www.dragonflydb.io/docs/getting-started/docker), [.sources/docker-install.md](.sources/docker-install.md); [GitHub release v1.40.2](https://github.com/dragonflydb/dragonfly/releases/tag/v1.40.2), [.sources/release-v1.40.2.md](.sources/release-v1.40.2.md))

Pin with an explicit semver tag (`:v1.40.2`), not `:latest`, for reproducible CI. Compatibility matrix on dragonflydb.io was verified against **Dragonfly v1.40.0** ([compatibility page](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility.md](.sources/compatibility.md)).

**Conflict:** Docker Hub mirror `dragonflydb/dragonfly` is stale (last tagged `v1.27.1`, updated >1 year ago). Use `docker.dragonflydb.io/dragonflydb/dragonfly` for current builds. ([Docker Hub tags](https://hub.docker.com/r/dragonflydb/dragonfly/tags), [.sources/dockerhub-tags.md](.sources/dockerhub-tags.md))

## Default port and redis-cli

- Listens on **6379** (same default as Redis).
- macOS/Windows/docker-compose: publish `-p 6379:6379`.
- Linux quick-start often uses `--network=host` instead of port mapping.

Official docs: "You can use the `redis-cli` to connect to `localhost:6379`." Example session: `set hello world` → `OK`, `get hello` → `"world"`. `PING` is listed as fully supported in the compatibility matrix. ([docker install](https://www.dragonflydb.io/docs/getting-started/docker), [.sources/docker-install.md](.sources/docker-install.md); [quick-start README in repo](https://github.com/dragonflydb/dragonfly/blob/main/docs/quick-start/README.md), [.sources/quick-start-readme.md](.sources/quick-start-readme.md); [compatibility — PING](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility.md](.sources/compatibility.md))

## Compose / CI gotchas (minimal e2e next to redis:7-alpine)

Official sample compose ([contrib/docker/docker-compose.yml](https://github.com/dragonflydb/dragonfly/blob/main/contrib/docker/docker-compose.yml), [.sources/docker-compose.yml.md](.sources/docker-compose.yml.md)):

```yaml
services:
  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    ulimits:
      memlock: -1
    ports:
      - "6379:6379"
```

Gotchas for side-by-side CI:

1. **Port collision** — both Redis and Dragonfly default to 6379. Map one service to a host port (e.g. Redis `6379:6379`, Dragonfly `6380:6379`) or use separate compose networks with internal DNS names (`redis:6379`, `dragonfly:6379`).
2. **`ulimits memlock: -1`** — recommended in every official `docker run`/compose example; omitting it can cause initialization failures on some hosts. Docs also note `--privileged` as a fallback for stubborn init errors. ([docker install](https://www.dragonflydb.io/docs/getting-started/docker), [.sources/docker-install.md](.sources/docker-install.md))
3. **`network_mode: host`** — best throughput on Linux but **not supported on Windows** and problematic on macOS; use `-p` mapping in CI runners that are not Linux host-network. ([contrib/docker README](https://github.com/dragonflydb/dragonfly/blob/main/contrib/docker/README.md), [.sources/contrib-docker-readme.md](.sources/contrib-docker-readme.md))
4. **Thread flags** — Dragonfly tests often pass `--proactor_threads=N` (e.g. `1`–`4`) for deterministic load; official minimal compose does **not** set it (server picks defaults). For constrained CI VMs, consider `command: ["--proactor_threads=1"]` if CPU contention appears — pattern from integration tests, not required by quick-start. ([eval_test.py proactor_threads](https://github.com/dragonflydb/dragonfly/blob/main/tests/dragonfly/eval_test.py), [.sources/eval-test-proactor.md](.sources/eval-test-proactor.md))
5. **RAM guidance** — docs recommend ≥4 GB RAM "to get the benefits of Dragonfly"; tiny e2e smoke tests usually run below that but may be slower. ([docker install prerequisites](https://www.dragonflydb.io/docs/getting-started/docker), [.sources/docker-install.md](.sources/docker-install.md))

Example minimal dual-backend snippet:

```yaml
services:
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2
    ulimits:
      memlock: -1
    ports: ["6380:6379"]
```
