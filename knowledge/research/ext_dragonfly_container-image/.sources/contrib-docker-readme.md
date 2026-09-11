---
url: https://github.com/dragonflydb/dragonfly/blob/main/contrib/docker/README.md
ref: dragonflydb/dragonfly@main:contrib/docker/README.md
title: Docker Compose README
fetched: 2026-09-11
authority: official
---

docker-compose up -d with official compose file.
Container image line: docker.dragonflydb.io/dragonflydb/dragonfly
Maps 0.0.0.0:6379->6379/tcp.

redis-cli to localhost:6379 works (set/get example).

Performance: host network_mode avoids docker NAT overhead; overlay network has NAT cost. network_mode host not supported in Swarm.
