# Standards

1. [hard] Leave a trail — `openspec/changes/retry-only-idempotent-commands/tasks.md:25` — a second `## 4. Specs` repeats 4.1/4.2 with empty checkboxes next to the checked copy, so the trail says those tasks both landed and did not
   ```
   ## 4. Specs
   - [x] 4.1 Confirm the change delta ...
   - [x] 4.2 Run `openspec validate --change retry-only-idempotent-commands --strict`

   ## 4. Specs
   - [ ] 4.1 Confirm the change delta ...
   - [ ] 4.2 Run `openspec validate --change retry-only-idempotent-commands --strict`
   ```
   → Delete the unchecked duplicate `## 4. Specs` block
   Status: done
   Argument: deleted the unchecked duplicate `## 4. Specs` block in tasks.md.
2. [hard] Leave a trail — `e2e/simpleredisprobe/plugin.go:69` — `ServeHTTP` now also warms the drop-relay client and sets DropIncr/DropEval headers, but the job comment still lists only the happy-path verbs
   ```
   // ServeHTTP runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval, then copies results into headers.
   func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
   	...
   	if m.dropClient != nil {
   		m.writeDropHeaders(rw, prefix)
   	}
   ```
   → Name the drop-relay Incr/Eval headers in the `ServeHTTP` comment (when `dropClient` is set)
   Status: done
   Argument: ServeHTTP comment now names DropIncr/DropEval headers when dropClient is set.
