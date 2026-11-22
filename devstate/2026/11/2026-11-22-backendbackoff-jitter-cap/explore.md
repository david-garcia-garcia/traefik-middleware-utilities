# Explore

## Concepts

- Units this change would touch:
  - `cooldownDuration` — `backendbackoff/gate.go:156-182` — doubles `BaseCooldown` up to `MaxCooldown`, then applies `wait * (1 + Jitter*(2u-1))` and clamps to `MaxCooldown`. This is the only production jitter body.
  - `Report` call sites — `backendbackoff/allow.go:72` (failed HALF-OPEN probe, `n++` then wait) and `backendbackoff/allow.go:86` (CLOSED trip) — store `now.Add(g.cooldownDuration(entry.n))` on `openUntil`. Signature unchanged if the body stays on this function.
  - Live contract `openspec/specs/std_go_backendbackoff_cooldown/spec.md` requirement "Cooldown opens and probes" — names `cooldown * (1 + Jitter * (2u - 1))` capped at `MaxCooldown`. Spec text must move with any formula change.
  - Live contract `openspec/specs/std_go_backendbackoff_allow/spec.md` — `Jitter` 0 SHALL disable jitter. Construction / defaults, not the spread formula.
  - Dest tests — `backendbackoff/zzz_gate_test.go` `newTestGate` (`Jitter: 0`) and two inline `Jitter: 0` configs; `backendbackoff/zzz_gate_yaegi_test.go` tripprobe; `backendbackoff/zzz_bench_test.go`. New proofs belong in a self-contained `zzz_*` file with helpers suffixed by this ticket slug.
  - Usage packet `knowledge/devdocs/std_go_backendbackoff.md` — exponential cooldown; does not name the point mass at `MaxCooldown`.

- How they connect:

```
Report (allow.go)
  HALF-OPEN probe fail  -->  n++; openUntil = now + cooldownDuration(n)
  CLOSED credit trip    -->  openUntil = now + cooldownDuration(n)
                                      |
                                      v
                         gate.go cooldownDuration(n)
                         wait = BaseCooldown * 2^n, cap Max
                         Jitter <= 0  --> return wait (frozen)
                         else jittered = wait * (1 + J*(2u-1))
                         if jittered > MaxCooldown --> MaxCooldown   <-- point mass
                                      |
                                      v
                         Allow OPEN deny: retryAfter = openUntil - now
```

- Call sites that matter (bounded vs structural):
  - Pattern `cooldownDuration`, roots searched: worktree `D:\repositories\traefik-middleware-utilities-2026-11-22-backendbackoff-jitter-cap` (all files). **4 hits in 2 files**: definition + comment `backendbackoff/gate.go:155-156`; **2 production callers** `backendbackoff/allow.go:72` and `:86`. No other package calls it. Tests in dest do not invoke it by name (they assert `retryAfter` with `Jitter: 0`).
  - Pattern `Jitter`, same root: **25 hits in 10 files**. Live (excluding `openspec/changes/archive/`): **20 hits in 6 files** — `backendbackoff/gate.go` 10, `zzz_gate_test.go` 5, `zzz_bench_test.go` 1, `zzz_gate_yaegi_test.go` 1, `openspec/specs/std_go_backendbackoff_cooldown/spec.md` 2, `openspec/specs/std_go_backendbackoff_allow/spec.md` 1. Archive copies: 5 hits in 4 files. No dest test samples the jitter distribution at the cap.

- Reproduce: **reproduced**. Command from worktreePath: `go test ./backendbackoff -run '^TestScale_JitterPointMassAtMax$' -count=1 -v` (throwaway `backendbackoff/zzz_explore_jitter_cap_throwaway_test.go`, copied only `TestScale_JitterPointMassAtMax` from the hunt repro, then deleted). Runtime `go1.25.6 windows/amd64`, module `go 1.21`.
  - Hunt loop (`Jitter` 0.1, `BaseCooldown` 1s, `MaxCooldown` 10s, `n=10`, 100_000 samples): **50.2%** exactly at `MaxCooldown`. Test itself **PASS** (log-only; no assertion of the mass). Hunt report §5 measured 50.0% on master `1f5ec454`.
  - (a) `Jitter` 0, `n=10`, 100_000 samples: **1** distinct duration, all `10s`. Never jitters.
  - (b) wait below cap, small `n` (`n=0` wait=1s): **0.0%** at `MaxCooldown`.
  - (c) wait just below cap with default knobs (`n=3` wait=8s): **0.0%** at `MaxCooldown`, 100_000 distinct Durations. Doubling jumps to Max on the next `n` (`gate.go:160-163`: if `wait > Max/2` set Max). `8s * 1.1 = 8.8s < 10s`, so default 1s/10s never clips below the cap.
  - Near-cap legal Config (`BaseCooldown=9.5s`, `Max=10s`, `J=0.1`, `n=0`): **23.9%** exactly at Max (matches the overshoot band `(10.45-10)/(10.45-8.55) ≈ 23.7%`). Clipping is not only `wait == Max`.
  - Duration truncation at cap: **49_609** unique Durations; mass at `Max-1ns` **0.0000%**. The 50% mass is the `if jittered > MaxCooldown { return MaxCooldown }` branch returning the config Duration, not `float64`→`time.Duration` binning of the unclamped half.

- Outside facts used: **in-tree**, no research folder. `math/rand.Float64` contract from GOROOT `math/rand/rand.go:188-211` (comment: half-open `[0.0,1.0)`; if the Go-1 stream rounds to `1.0` it resamples). Throwaway inside the same deleted test: 1_000_000 samples, `min>0`, `max=0.99999925952900448`, `saw1=false`, `sawNeg=false`. Index `knowledge/research/index_ext_go.md` has no Float64 packet; none written. Yaegi constraints from `requirement.md` Constraints and `knowledge/research/index_ext_traefik.md` (select / unsafe / generics packets already exist). Hunt report read-only §5.

- OPTIONS for the jitter-at-cap formula (not a chosen seam). All stay Yaegi-safe if they keep arithmetic + `rng.Float64()` on the existing per-Gate source (already under `g.mu` at Report): no `select` mixing per-call channels, no `unsafe`, no `container/list` any-typed `Value`, no package-level generic instantiation aliases.

  1. **Hunt-report suggestion** — when nominal `wait` is already `MaxCooldown`, sample uniform on `[Max*(1-J), Max]` (or the half-open equivalent with `u ∈ [0,1)`). Below-cap keeps today's `wait * (1 + J*(2u-1))`. Tradeoff: smallest change at the failing case; **does not** remove the measured 23.9% near-cap mass when `wait < Max` but `wait*(1+J) > Max`. Spec must stop saying "capped" as clamp-after-symmetric-jitter for the at-cap branch.

  2. **Interval-first, then sample** — build `[wait*(1-J), min(wait*(1+J), Max)]` and draw uniform inside it (pre-clamp the *range*, not the already-jittered sample). Tradeoff: also spreads near-cap (covers the 9.5s Config); changes the density of the upper tail whenever the +J side would exceed Max; still one `Float64` draw, no loop. If implemented as clamp-after-sample into that interval, the mass at Max returns — the interval must be the support of the draw.

  3. **Reject-and-resample** until `jittered <= Max`. Equivalent in law to uniform on `[wait*(1-J), Max]` when the +J side overshoots. Tradeoff: no new branch on `wait == Max`; expected draws `1/P(accept)` (~2 at cap with J=0.1); theoretically unbounded loop, but with `wait <= Max` and `J ∈ [0,1)` the lower endpoint is always `<= Max`, so each draw has P(accept) ≥ 0.5 at cap. Yaegi-safe (plain `for`). Extra `Float64` calls vs (1)/(2).

  4. **Keep clamp, change only comment/spec** — leave `gate.go:178-179`. Tradeoff: zero code risk, matches today's live spec letter; **fails Desired** ("no single value carries a point mass at the cap"). Measured 50.2% at Max falsifies this as a complete fix.

  Extra (not in the hunt note): AWS-style full jitter `uniform[0, min(wait, Max)]` would destroy the ±J band below the cap and the Jitter-0-except-when-positive contract shape. Desired does not ask to retune the below-cap spread. Listed so propose can reject it with eyes open.

## Decisions

- No chosen seam this phase (tech lead propose). Options (1)–(4) stay options. The formula Q is not assumed to a specific interval such as uniform-on-`[Max*(1-J), Max]`.

- Rejected alternatives (out of scope or falsified), one line each:
  - `loadEntry` O(n) map scan — out of scope (sibling `backendbackoff-new-key-scan`).
  - `Report` without identity / stale HALF-OPEN probe — out of scope (sibling `backendbackoff-stale-probe-report`).
  - `iplookup`, `windowcounter`, `reclaim`, `tokenbucket`, `simpleredis` — out of scope.
  - Shared helper files across the seven PRs — out of scope.
  - Changing Yaegi restrictions — out of scope.
  - Picking the final sampling algorithm in explore — out of scope (process note).
  - Shared-replica backoff layer (`knowledge/debt/2026-09-12-backendbackoff-shared-layer.md`) — out of scope.
  - Changing `FailureRatio`, `TripFailures`, `TTL`, the probe lease, or `n` reset after `MaxCooldown` of CLOSED — out of scope.
  - `simpleredis` retry jitter — out of scope.
  - Keep clamp / comment-and-spec only as the delivery of Desired — falsified: Desired forbids a point mass at the cap; this worktree measured 50.2% at `MaxCooldown`.

- Live contract: `openspec/specs/std_go_backendbackoff_cooldown` (formula + cap letter) and `openspec/specs/std_go_backendbackoff_allow` for `Jitter` 0 disables jitter. Propose writes the OpenSpec change; this phase does not.

## Open questions

- Q: Which jitter-at-cap formula meets Desired (never exceed Max, no point mass at the cap, Jitter 0 still disables)?
  Rank: bounded asked — `cooldownDuration` already has 2 production callers (`backendbackoff/allow.go:72`, `:86`; roots: worktree, pattern `cooldownDuration`, 4 hits / 2 files) and this change would migrate them here; Desired names "no single value carries a point mass at the cap; jittered probes stay spread when the nominal cooldown is at the cap".
  Decision: resolved — by propose, pointing to design.md (pre-bounded interval, option 2)
  By: propose

- Q: Should the chosen formula also spread when `wait < Max` but `wait*(1+J) > Max` (near-cap clip), not only when `wait == Max`?
  Rank: bounded asked — same `cooldownDuration` callers (2, roots as above); Unknowns names below-cap overshoot; Desired's "no single value carries a point mass at the cap" is true of that clip too.
  Decision: resolved — by propose, pointing to design.md (cover the near-cap band with option 2; option 1 rejected because it leaves the 23.9% mass)
  By: propose

- Q: Does `float64` → `time.Duration` truncation create extra point masses besides the Max clamp?
  Rank: additive incidental — conversion already sits inside `cooldownDuration`; Desired does not name truncation; no new callers.
  Decision: resolved — at 1s/10s cap, mass at `Max-1ns` is 0.0000%; ~49_609 unique Durations in 100_000 draws (unclamped half is 1 ns-wide bins, not a mass). The 50% mass is `return g.cfg.MaxCooldown`. Propose still must not reintroduce a clamp-after-sample onto one Duration.
  By: explore

- Q: Is `rng.Float64()` exactly `[0, 1)` as the cooldown spec writes for `u`?
  Rank: additive asked — spec `openspec/specs/std_go_backendbackoff_cooldown/spec.md:8` names `u` in `[0, 1)`; Unknowns repeats it.
  Decision: resolved — GOROOT `math/rand/rand.go:188-211` documents `[0.0,1.0)` and resamples a rounded `1.0`; 1_000_000 draws in the throwaway never hit 1 or a negative. No `knowledge/research` packet (in-tree stdlib).
  By: explore

- Q: Which dest tests break if `newTestGate` keeps `Jitter: 0` after the formula change?
  Rank: bounded asked — Constraints require dest tests that freeze cooldowns to keep working; `newTestGate` is the dest helper (`zzz_gate_test.go:402-410`).
  Decision: resolved — none of the dest cooldown asserts break if `Jitter` stays 0. `newTestGate` is used by 13 tests; exact `retryAfter` from `cooldownDuration` with Jitter 0: `TestReport_ConsecutiveFailuresTrip` (1s), `TestReport_OpenIgnored` (1s), `TestReport_ProbeSuccessRetainsN` (2s), `TestReport_ProbeFailureIncrementsN` (2s), `TestAllow_NResetsAfterMaxClosed` (1s), `TestAllow_EarlyRetripKeepsN` (2s). `TestAllow_IdleKeyPresumedHealthy` and `TestAllow_DenyRefreshesTTL` set `Jitter: 0` inline. `TestAllow_ProbeAfterCooldown` asserts the probe lease (`BaseCooldown` in `allow.go:28,35`), not `cooldownDuration`. Yaegi tripprobe and the warm Allow bench also pass `Jitter: 0`. `TestNew_ZeroConfigDefaults` still wants `defaultJitter` 0.10 on a fully zero Config. New distribution tests must not share helpers with sibling PRs (slug suffix).
  By: explore

- Q: How to isolate hunks against sibling `backendbackoff-*` tickets (`new-key-scan`, `stale-probe-report`) on the same package?
  Rank: bounded asked — Constraints: "Touch only what this ticket needs" and "keep hunks minimal so merges stay mechanical"; three `backendbackoff-*` tickets of the batch.
  Decision: resolved — by propose, pointing to design.md (product hunk stays in `cooldownDuration`; `allow.go` untouched)
  By: propose

- Q: `knowledge/devdocs/std_go_backendbackoff.md` does not mention the point mass at `MaxCooldown`; should usage/Gotchas name it after the formula change?
  Rank: additive incidental — packet usage is enough to call Allow/Report today; the gap is a gotcha the change would make stale or newly true; requirement does not name the packet. Attended: do not write `knowledge/devdocs` without human yes.
  Decision: assumed — park; explore does not write the packet. Propose/devdocsimpact decide the one-line gotcha once the formula is chosen.
  By: explore
