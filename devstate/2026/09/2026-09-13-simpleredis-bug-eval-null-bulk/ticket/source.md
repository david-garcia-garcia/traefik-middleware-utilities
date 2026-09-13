# Eval maps Lua false/nil (RESP2 top-level $-1) to redis:miss

Get: null bulk SHALL be redis:miss. Eval: return [][]byte slots or a Lua/server - error. Decode maps every top-level $-1 to ErrMiss. Lua return false / return nil is RESP2 null bulk (not a stored key). Callers that treat IsMiss as “key absent” treat a successful Eval false as a Get miss. return {false} is already an array nil slot with nil error. SET cannot store Redis nil/false; this is only the EVAL reply.

Cause: readBulk length<0 → errMiss; readReply top-level $ forwards errMiss; array $ continue leaves values[i]==nil. Get forwards exec’s error and never sees a nil slot.

Agreed how (do not invent another):
- Null bulk is a nil slot, not ErrMiss from decode.
- readBulk may still use errMiss internally for length < 0.
- readReply top-level $ null: return [][]byte{nil}, true, nil (same as an array null slot).
- Get: after a 1-slot reply, values[0]==nil → errMiss. Empty bulk $0 stays a non-nil empty slice (not a miss).
- Eval return false → [][]byte{nil}, nil. errors.Is(err, ErrMiss) must be false.
- Do NOT remap ErrMiss only inside Eval (papers over decode). MGET aligned nil slots stay as they are.
- Update specs if decode currently says $-1 remains redis:miss for every verb — Get still maps miss; decode yields a nil slot.

Out of scope: handshake redial, Eval/MSetEX per-hop deadlines.

Implement order (human override; required):
1. FIRST land compiled tests that reproduce (MUST fail on current dest). No //go:build bugrepro — default go test -short ./simpleredis/.
2. THEN apply the agreed how.
3. THEN confirm those tests pass AND Get miss still works AND MGET nil slots AND $0 empty bulk is not miss.

Reuse startStaticRedis in simpleredis/fake_redis_test.go.

Example repro (adapt into commands_eval_test.go or resp_test.go):

```go
func TestEvalNullBulkIsNotMiss(t *testing.T) {
	addr := startStaticRedis(t, "$-1\\r\\n")
	client := New(Config{Host: addr, MaxRetries: -1})
	values, err := client.Eval(context.Background(), "return false", nil, nil)
	if errors.Is(err, ErrMiss) {
		t.Fatalf("Eval $-1: values=%q err=%v, want a result slot not redis:miss", values, err)
	}
	if err != nil { t.Fatalf("Eval $-1: err=%v, want nil error", err) }
	if len(values) != 1 || values[0] != nil {
		t.Fatalf("Eval $-1: values=%q, want one nil slot", values)
	}
}
```
Also add Get $-1 still ErrMiss, and $0 Get not miss, once decode changes (those may be existing tests — keep them passing).
