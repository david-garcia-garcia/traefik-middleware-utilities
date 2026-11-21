---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/82498271f7f15b4aa128bb3e13fc015e40242e6f/pkg/lapi/poll_timer.go
title: crowdsec-bouncer-traefik-plugin pollTimer (PR 208)
fetched: 2026-11-21
authority: source
ref: github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin@82498271f7f15b4aa128bb3e13fc015e40242e6f:pkg/lapi/poll_timer.go
---

Doc comment: Traefik runs the plugin under yaegi v0.16.1, "which builds one case list per select statement and shares it across every goroutine that runs that statement". Loops of different Clients running the same select "could take each other's ticks or stop signals"; a Client could stop polling for good while reporting healthy, with no error logged.

Fix: `pollTimer` has no `select`. `startPollTimerEvery` arms `time.AfterFunc(interval, fire)` while holding `mu`. `fire` returns if stopped, runs `work`, then `timer.Reset(interval)` under `mu` unless stopped (fixed delay, no overlap). `stop` is nil-safe and idempotent: sets `stopped` and calls `timer.Stop()` under `mu`; it does not wait for running work.

Comment also notes: upstream (maxlerebourg PR 399) used `for range ticker.C { go work() }`; rejected here because `Ticker.Stop` does not close `C`, so a ranging goroutine would block forever and keep the Client alive. A `time.Sleep` loop was rejected because its goroutine outlives stop by up to one interval.
