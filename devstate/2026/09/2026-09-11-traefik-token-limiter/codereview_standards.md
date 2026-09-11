# Standards

1. [hard] Leave a trail — `tokenbucket/redis.go:65` — `ParseFloat` failure on the Eval wait field returns `errEvalLen` (`eval reply must have 3 fields`), so a 3-field reply with a non-numeric wait is classified as a length error
   → Return a parse-specific error (or `convErr`); keep `errEvalLen` for `len != 3` only
   Status: done
   Argument: return errEvalWait when wait is not numeric.
