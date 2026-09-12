# test-05 — exec retries INCR/INCRBY/EVAL after a lost reply

issueHost: local
issueRef: none
Finding: simpleredisfixes/test-05-non-idempotent-retry.md
Index: simpleredisfixes/README.md

## Caller spec

Policy (unattended assumed unless dest evidence contradicts): retry only idempotent commands; INCR/INCRBY/EVAL return redis:unreachable instead of double-applying. Write the Decision on explore.md. Implement the chosen policy and the tests in How to prove it.

HARD REQUIREMENT: tests MUST run against both Redis and Dragonfly. Both are supported backends. Prove Incr/Eval retry policy on both live engines as well as the fake. Extend compose + Pester `/redis` `/dragonfly`. Lua 5.1-safe. Dragonfly KEYS required.

DestBranch: master (human confirmed).

## Finding

# test-05 — `exec` retries `INCR`/`INCRBY`/`EVAL` after a lost reply

- **Axis**: Test coverage (with a design decision attached)
- **Severity**: hard
- **Where**: `simpleredis/simpleredis.go:182-201` (`exec`)
- **Status**: not applied — **needs a policy decision before the rate limiters land**

## What I found

`exec` retries any command, regardless of whether it is safe to repeat.

The retry fires when the command failed on a **reused** connection with a non-timeout
error. That condition is correct for detecting a dead pooled socket, but it cannot
distinguish two very different situations:

1. The write never reached Redis — retrying is correct and invisible.
2. The write reached Redis, Redis **executed** the command, and the reply was lost
   (connection died between execution and the client finishing its read).

In case 2, `INCR`, `INCRBY` and `EVAL` are re-sent and **applied twice**. The
truncated-reply path in test-04 is a concrete way to reach case 2, and no test pins
the behaviour either way.

## Why it matters

The consumers being built on this library are counters. A silent double-apply means
rate limiters that over-count and flush deltas applied twice. Idempotent verbs
(`GET`, `MGET`, `SET`, `DEL`, `EXPIRE`, `EXPIREAT`) are fine to retry. Only the
counter and script verbs are affected.

## How to fix

Pick a policy and test it. In rough order of preference:

- **Retry only idempotent commands.** Tag each verb at its call site (a bool or a
  small command descriptor passed into `exec`) and skip the retry for `INCR`,
  `INCRBY` and `EVAL`, returning `redis:unreachable` instead. The caller then decides
  whether to retry with full knowledge. This keeps the transparent-recovery benefit
  for the read-heavy verbs, which are the ones that actually hit stale pooled
  connections in practice.
- **Retry only when the write provably did not land.** Distinguish a failure during
  the write/flush (safe to retry) from a failure during the read (not safe). This is
  more precise, but a write into a socket buffer can still be delivered after the
  local call succeeds, so it does not fully close the window — it narrows it.
- **Make it a caller decision.** An option on the client, defaulting to the
  conservative behaviour.
- **Keep retrying everything**, and document the double-apply risk explicitly so the
  limiters can compensate — for example by making their scripts idempotent with a
  caller-supplied request token. Acceptable only if written down; the current state
  is that the risk is neither prevented nor documented.

Whatever is chosen, the same reasoning must be applied to the pipeline in
perf-04, which must not be retried wholesale.

## How to prove it

A fake that executes the command, mutates its store, then closes the connection
before writing the reply. Assert the resulting stored value: with the recommended
policy, one `Incr` produces `1` and an error rather than `2` silently. Add the
mirror-image test for an idempotent verb, asserting the retry still happens and the
caller sees success. Extend the fake used in test-04 so the same harness serves both.
