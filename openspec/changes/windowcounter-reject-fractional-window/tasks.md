## 1. Failing repro first

- [ ] 1.1 Create `windowcounter/repro_fractional_window_test.go` `TestRepro_FractionalWindowAccepted` with subtest A: 1500ms Take MUST error (fake Redis so dest today succeeds)
- [ ] 1.2 Confirm FAIL on unfixed dest (`go test -short -count=1 -timeout 60s ./windowcounter -run TestRepro_FractionalWindowAccepted`): subtest A fails because Take succeeds
- [ ] 1.3 Keep `TestTake_SubSecondWindow` (500ms still errors)

## 2. slidingAt gate

- [ ] 2.1 In `slidingAt`, error if `window < time.Second` (existing string) **or** `window % time.Second != 0` (`windowcounter: window must be a whole number of seconds`). Do not truncate
- [ ] 2.2 Weight denom from `windowSec` only (`float64(windowSec)*float64(time.Second)` or equivalent); TTL stays `2 * windowSec`
- [ ] 2.3 Confirm repro PASSES (subtest A: 1500ms Take errors)

## 3. Package tests and usage

- [ ] 3.1 `go test -short -count=1 -timeout 60s ./windowcounter` passes
- [ ] 3.2 Usage gotcha: Take/Peek reject a non-whole-second window; they do not truncate (`knowledge/devdocs/std_go_windowcounter.md`)

## 4. Validate

- [ ] 4.1 Run `openspec validate --change windowcounter-reject-fractional-window --strict`
