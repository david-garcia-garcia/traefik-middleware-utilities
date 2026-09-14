# Nitpicks

1. [hard] Symmetry and consistency — `reclaim/table.go:307` — `lookupOpen` returns `openStep{action: openPark|…}` while sibling lock-and-decide helpers `claimDrop` / `claimExpire` encode the same role with `ready`/`stop`/`skip`/`enforce` flags so `drop` must infer park vs stop vs claim from `step.ready != nil` and `step.stop`
   → Add a `dropAction` (and mirror for expire) like `openAction`, set it in the helpers, and switch in `drop` / `expire` the way `Open` switches on `step.action`
   Status: done
   Argument: dropAction/expireAction plus switch in drop/expire.

2. [hard] Clear conditions — `reclaim/table.go:534` — after the `slotBusy` branch, `if incarnation.holders > 0 || incarnation.state != slotAwake { return dropStep{stop: true} }` hides that only `holders == 0 && state == slotAwake` enters the last-holder Sleep claim
   → Guard the claim body with `incarnation.holders == 0 && incarnation.state == slotAwake` (else `stop: true`)
   Status: done
   Argument: claimDrop guards last-holder with holders == 0 && state == slotAwake.

3. [hard] Clear conditions — `reclaim/table.go:641` — `if incarnation.state != slotAsleep || incarnation.holders > 0 { return expireStep{skip: true} }` names the complement instead of the asleep, zero-holder case this body is about to take
   → Guard the expire claim with `incarnation.state == slotAsleep && incarnation.holders == 0` (else `skip: true`)
   Status: done
   Argument: claimExpire guards with state == slotAsleep && holders == 0.
