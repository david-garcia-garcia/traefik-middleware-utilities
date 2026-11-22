# Standards

1. [hard] Bound the ask — `knowledge/opendev.json:1` — the product diff adds OpenDev local settings (`model_default`, languages) and `.cursorignore`, which local settings tooling emits and which are unrelated to dropped-client GC or Traefik #13618
   Fix: Remove both files from the branch (or land them in their own change if the repo versions them)
   Status: done
   Argument: Removed `.cursorignore` and `knowledge/opendev.json` from the branch.
   Quote:
      ```json
      {
        "model_default": "(omitted)",
      ```
2. [hard] Name for the scope — `tokenbucket/traefik_issue13618_e2e_test.go:18` — `countingProxy13618` is a test-only type whose name does not say test; package siblings are `testFakeRedis` and `liveEngineForTest`
   Fix: Rename to `testCountingProxy13618` and match the constructor (`startTestCountingProxy13618`); keep the 13618 suffix
   Status: done
   Argument: Renamed `countingProxy13618` to `testCountingProxy13618` and `startCountingProxy13618` to `startTestCountingProxy13618`.
   Quote:
      ```go
      type countingProxy13618 struct {
      	listener net.Listener
      	open     atomic.Int64
      }
      ```
