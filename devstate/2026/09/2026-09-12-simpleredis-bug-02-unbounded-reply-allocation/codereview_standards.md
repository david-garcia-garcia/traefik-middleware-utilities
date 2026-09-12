# Standards

1. [hard] Leave a trail — `simpleredis/resp.go:81` — new `*` over-cap gate has no block intro; a later short `readLine` is still EOF → `redis:unreachable` and retries
   ```
   		if count > maxArrayCount {
   			return nil, false, errIssue
   		}
   		values := make([][]byte, count)
   ```
   → One-line intro before the gate: over-cap `*` must not `make`; a later short read retries as unreachable
   Status: done
   Argument: one-line intro before the `*` gate in `simpleredis/resp.go`.
