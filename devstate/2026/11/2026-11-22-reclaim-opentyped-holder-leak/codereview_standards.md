# Standards

1. [hard] Leave a trail — `reclaim/opentyped.go:9` — OpenTyped's comment still equates it to OpenWithHooks; mismatch now releases the holder
   Fix: Say a type mismatch returns the error and does not keep the caller as a holder
   Status: done
   Argument: OpenTyped godoc now says a type mismatch returns the error and does not keep the caller as a holder.
   Quote:
      ```
      // OpenTyped is OpenWithHooks with the stored value returned as T, so a caller that knows what it
      // created does not repeat the same type assert on every Open.
      ...
      	typed, ok := stored.(T)
      	if !ok {
      		t.releaseLease(lease)
      		return zero, fmt.Errorf("reclaim: open %q: want %T, got %T", key, zero, stored)
      	}
      ```
