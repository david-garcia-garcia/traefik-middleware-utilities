# Leaky bucket as a meter

Classic water-level meter (pour, leak, overflow deny). Not a FIFO queue. Not Traefik RateLimit token bucket (`ext_traefik_ratelimiter_token-bucket`). Not Kong sliding windows (`ext_kong_rate-limiting_sliding-sync`).

## Clock

Ticket clock, same meter Wikipedia and ITU-T name:

`water(t) = max(0, water(t0) + poured − leak×Δt)`, then cap `capacity`. Overflow → deny and **do not pour**. Empty bucket = room to burst up to `capacity`; then wait for leak. `tokens ≈ capacity − water` is remaining room, not the stored state.

Wikipedia’s **meter** (not the queue): a counter separate from the flow, incremented on an event, decremented at a fixed rate; if adding the event would overflow, the event does not conform and the water is left unchanged. Empty stops leaking. ([Leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket), [.sources/leaky-bucket.md](.sources/leaky-bucket.md))

ITU-T I.371 Annex A.2 is the same meter as GCRA’s continuous-state leaky bucket (`official`). Finite-capacity real-valued content drains at 1 unit per time unit; a conforming cell adds increment `T`. At arrival, drain first: `X' = X − (ta − LCT)`. If `X' ≤ τ`, conforming and `X = max(0, X') + T`, `LCT = ta`. If `X' > τ`, non-conforming and `X`/`LCT` unchanged. Capacity is `T + τ`. First cell: `X = 0`. ([I.371 (03/2004) Annex A](https://www.itu.int/rec/T-REC-I.371-200403-I/en), [.sources/i371-annex-a.md](.sources/i371-annex-a.md))

Map, do not replace the ticket formula: I.371 leak rate is 1 (content units = time); ticket `leak` is water per time. I.371 `T` is pour per conforming event; `τ` is the limit before pouring; capacity `T + τ`. Ticket `cap` is implicit in I.371 because a pour is refused when `X' > τ`, so water never exceeds `T + τ`.

## Meter vs queue

Wikipedia names two algorithms both called leaky bucket. **Meter** (this folder, Turner / I.371): water is a check; packets do not sit in the bucket. **Queue** (Tanenbaum): the bucket *is* a FIFO serviced at a fixed rate; that is traffic shaping, not this clock. ([Leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket), [.sources/leaky-bucket.md](.sources/leaky-bucket.md))

## Token-bucket mirror (comment only)

Wikipedia: the meter is a mirror of a token bucket — adding water ↔ removing tokens, leaking ↔ adding tokens, overflow ↔ underflow — so equivalent parameters admit the same sequence. I.371 does **not** define tokens. Follow **official** for what the meter stores (water, leak, finite capacity, no pour on overflow). The mirror may color `tokens ≈ capacity − water`; it does not make this Traefik’s `x/time/rate` token bucket. ([Leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket); I.371 as above)

## Turner vs finite cap

Wikipedia: Turner (1986) increments then discards if the counter exceeds a threshold, and does not explicitly keep the counter finite. I.371 (and Wikipedia’s own concept-of-operation) refuse the pour that would overflow. Conflict → follow **I.371** (`official`): deny and leave water unchanged. That is the ticket’s overflow rule. ([Leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket); [I.371 Annex A.2](https://www.itu.int/rec/T-REC-I.371-200403-I/en))
