---
url: https://github.com/dragonflydb/dragonfly/blob/main/contrib/docker/docker-compose.yml
ref: dragonflydb/dragonfly@main:contrib/docker/docker-compose.yml
title: Official docker-compose.yml
fetched: 2026-09-11
authority: official
---

services:
  dragonfly:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    ulimits:
      memlock: -1
    ports:
      - "6379:6379"
    # network_mode: "host" commented — not supported in Swarm; host mode faster on Linux
    volumes:
      - dragonflydata:/data
