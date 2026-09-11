---
url: https://www.dragonflydb.io/docs/getting-started/docker
title: Install with Docker
fetched: 2026-09-11
authority: official
---

Image: docker.dragonflydb.io/dragonflydb/dragonfly

Linux: docker run --network=host --ulimit memlock=-1 docker.dragonflydb.io/dragonflydb/dragonfly

macOS/Windows: docker run -p 6379:6379 --ulimit memlock=-1 docker.dragonflydb.io/dragonflydb/dragonfly

Prerequisites: minimum 4GB RAM, 1 CPU core, Linux kernel 4.19+.

Dragonfly responds to http and redis requests. Use redis-cli to connect to localhost:6379.

Note: docker run --privileged may fix initialization errors on some configurations.

Example: set hello world → OK; get hello → "world".
