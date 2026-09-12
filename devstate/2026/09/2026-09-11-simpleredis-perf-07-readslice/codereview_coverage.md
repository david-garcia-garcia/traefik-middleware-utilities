# Test coverage

1. [hard] Edge case untested — `simpleredis/simpleredis.go:454` — `parseLen` rejects overflow with `n > (maxParseLen-digit)/10`; `TestParseLen` has no input that hits that branch (`simpleredis/simpleredis_test.go:906`)
   → Add a digits string that overflows `int` (for example forty `9`s) and assert `ok == false`
   Status: done
   Argument: overflow digits case in TestParseLen (3c1ec7c).
