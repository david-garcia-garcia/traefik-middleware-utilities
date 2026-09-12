---
url: https://en.wikipedia.org/wiki/Leaky_bucket
title: Leaky bucket
fetched: 2026-09-11
authority: comment
---

Two algorithms share the name. Meter: a counter apart from the flow, incremented on an event, decremented at a fixed rate; overflow means non-conforming. Queue: the bucket is a FIFO serviced at a fixed rate (Tanenbaum); packets themselves are the water.

Meter concept of operation: fixed-capacity bucket, leaks at a fixed rate, stops leaking when empty. A packet conforms only if a specific amount of water can be added without overflow. If it would overflow, the packet does not conform and the water is left unchanged. Amount added may be fixed per packet or proportional to length.

ITU-T I.371 / ATM Forum GCRA quoted as the same meter: drain 1 unit per time unit, add T per conforming cell; if content > τ at arrival, non-conforming and bucket unchanged. Capacity = T + τ.

Turner (1986): increment on send, decrement periodically, discard if counter exceeds threshold. Wikipedia notes Turner does not explicitly keep the counter finite or skip the increment on overflow.

Meter is a mirror of token bucket (add water ↔ remove tokens; leak ↔ add tokens; overflow ↔ underflow). Equivalent parameters admit the same sequence. Queue form is a special case of the meter used for shaping (τ = 0, test at emission ticks).
