# Nitpicks

1. [hard] Name for the scope — `simpleredis/simpleredis_test.go:646` — `TestCloseDrainsIdleAndDoesNotRepool` still names repool after this change dropped the vacuous post-Get idle assertion; the remaining body is drain, unreachable, no redial
   → Rename to `TestCloseDrainsIdleAndDoesNotRedial`
   Status: done
   Argument: renamed TestCloseDrainsIdleAndDoesNotRedial (436cdd0).
2. [hard] Name for the scope — `scripts/integration-tests.Tests.ps1:43` — `Invoke-OverlappingWhoami` names the whoami service; the body fires 16 parallel requests at the caller Path (`/redis`, `/dragonfly`)
   → Rename to `Invoke-OverlappingRequests`
   Status: done
   Argument: renamed Invoke-OverlappingRequests (436cdd0).
