# Deviations

- [x] taken  keep Allow's Duration return instead of dropping it with waitDuration
  Asked: delete `waitDuration` / `allowedFromWait` rather than keep a conversion Allow no longer returns (caller dirty tree was `(bool, error)`).
  Instead: keep dest `(bool, time.Duration, error)`; delete `allowedFromWait`; delete named `waitDuration`; inline the conversion only for the second return.
  Owner: `tokenbucket/memory.go` `Memory.Allow` / `tokenbucket/redis.go` `Redis.Allow` (`openspec/specs/std_go_tokenbucket_allow/spec.md`)
  Why: dest spec and enumerated callers (`tokenbucket/limiter_test.go`, `limiter_e2e_test.go`, `limiter_yaegi_test.go`, `README.md`, `knowledge/devdocs/std_go_tokenbucket.md`) already return wait; the fail-open fix is the microseconds admit bool, not dropping wait.
  By: explore
  Requester: not asked
