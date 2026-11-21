# Devdocs impact
change: ratelimit-status

## Units
- Token bucket Status — subsystem — `tokenbucket/status.go`, spec `std_go_tokenbucket_status`, packet `std_go_tokenbucket`
- Window counter Status — subsystem — `windowcounter/status.go`, spec `std_go_windowcounter_status`, packet `std_go_windowcounter`

## Findings
- [x] language-gap  Token bucket Status — `std_go_tokenbucket` has How-to, snippet and Gotchas for `Status`, but no `Status` / `Remaining` Language term; **Allow** still said "returns whether the consume is allowed and how long to wait"; **Token bucket** `_Avoid_` rejected "remaining quota", which now contradicts `Status.Remaining`
- [x] language-gap  Window counter Status — `std_go_windowcounter` has How-to, snippet and Gotchas for `Status`, but no `Status` / `Remaining` Language term; **Window counter** and **Take** still said they return allow/deny plus the estimate; "remaining quota" was in `_Avoid_` of **Window counter**, **Peek** and **Take**

Usage (How to use, Pattern snippet, Key files, Gotchas) of both packets was already updated by implement (tasks 2.7, 3.7) and matches the apply: no `stale-usage`. No new package, no `missing-packet`, no `wrong-fold`.

## Produced
- `knowledge/devdocs/std_go_tokenbucket.md` — **Allow** returns a `Status`; new **Status** and **Remaining** terms; "remaining quota" dropped from **Token bucket** `_Avoid_`.
- `knowledge/devdocs/std_go_windowcounter.md` — **Window counter** and **Take** return a `Status`; new **Status** and **Remaining** terms; "remaining quota" dropped from `_Avoid_` of **Window counter**, **Peek**, **Take** (Peek-as-Allow and Peek-as-reservation avoids kept).
