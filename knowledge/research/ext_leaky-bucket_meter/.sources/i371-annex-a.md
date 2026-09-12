---
url: https://www.itu.int/rec/T-REC-I.371-200403-I/en
title: ITU-T Recommendation I.371 (03/2004) Traffic control and congestion control in B-ISDN
fetched: 2026-09-11
authority: official
---

Annex A defines GCRA(T, τ) used to test cell conformance to rate Λ = 1/T with tolerance τ. T and τ are in units of time.

Two equivalent versions: virtual scheduling (TAT) and continuous-state leaky bucket. Same arrival sequence → same conforming / non-conforming cells. After each arrival, TAT = X + LCT.

Continuous-state leaky bucket (A.2): finite-capacity bucket; real-valued content drains at 1 unit of content per time unit; content increased by increment T for each conforming cell. If at a cell arrival the content is ≤ limit τ, the cell is conforming; otherwise non-conforming. Capacity (upper bound of the counter) is (T + τ).

First cell ta(1): X = 0, LCT = ta(1). At ta(k): provisionally X' = X − (ta(k) − LCT) (drain since last conforming cell). If X' ≤ τ: conforming; X = max(0, X') + T; LCT = ta(k). If X' > τ: non-conforming; X and LCT unchanged.

Virtual scheduling (A.1): TAT starts at ta(1). Non-conforming if ta(k) < TAT − τ (TAT unchanged). Conforming and early (TAT − τ ≤ ta(k) < TAT): TAT += T. Conforming and late (ta(k) > TAT): TAT = ta(k) + T.
