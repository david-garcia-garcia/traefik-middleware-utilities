# Nitpicks

1. [hard] Name for the scope — `simpleredis/simpleredis.go:447` — `parseLen` locals `n` and `d` hide the accumulating length and the current digit byte
   → Rename to `length` and `digitByte`
   Status: done
   Argument: renamed `n`→`length` and `d`→`digitByte` (3c1ec7c).
