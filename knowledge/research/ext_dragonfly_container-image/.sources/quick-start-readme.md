---
url: https://github.com/dragonflydb/dragonfly/blob/main/docs/quick-start/README.md
ref: dragonflydb/dragonfly@main:docs/quick-start/README.md
title: Quick Start README
fetched: 2026-09-11
authority: official
---

docker run --network=host --ulimit memlock=-1 docker.dragonflydb.io/dragonflydb/dragonfly  # Linux
docker run -p 6379:6379 --ulimit memlock=-1 ...  # macOS
wslc run -p 6379:6379 --ulimit memlock=-1 ...  # Windows

redis-cli → localhost:6379. Optional --privileged for init errors.

network=host doesn't work on Windows; macOS issue linked.
