# Standards

1. [hard] Leave a trail — `simpleredis/simpleredis.go:233` — `execPipeline` retries on a dead reused idle socket with no block intro on the second borrow; sibling `exec` names that block (`Dead pooled conn: borrow again so Close cannot skip the closed check`)
   ```
   // After Flush, or on timeout, do not send the batch again (double-apply).
   if err == nil || reusable || !reused || err == errTimeout || flushed {
   	return slots, err
   }
   conn, _, err = sr.borrow()
   ```
   → Introduce the retry-borrow block with that job (dead pooled conn; borrow again so Close cannot skip the closed check)
   Status: done
   Argument: added the sibling `exec` intro on the retry-borrow (d044e6e).
