---
url: https://www.dragonflydb.io/docs/managing-dragonfly/flags
title: Server Configuration Flags
fetched: 2026-09-11
authority: official
---

--pipeline_buffer_limit: Amount of memory to use for storing pipeline requests per IO thread. Clients that send excessively huge pipelines may deadlock themselves. See GitHub discussion 3997. Default: 128.00MiB.

--pipeline_queue_limit: Pipeline queue max length. The server will stop reading from the client socket once its pipeline queue crosses this limit, and will resume once it processes excessive requests, to prevent OOM. Users of huge pipeline sizes may require increasing this limit to prevent the risk of deadlocking. See discussion 3997. Default: 10000.

--pipeline_squash: Number of queued pipelined commands above which squashing is enabled, 0 means disabled. Default: 1.

--enable_pipeline_squashing_v2: Enable vectorized pipeline squashing for the V2 dispatch loop. Default: true.
