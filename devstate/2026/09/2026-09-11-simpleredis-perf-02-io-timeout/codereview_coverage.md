# Test coverage

1. [hard] Edge case untested — `simpleredis/simpleredis.go:106-120` — `<= 0` copies package constants; `TestIoTimeout` (`simpleredis/simpleredis_test.go:588`) would still pass if `ioTimeout` stayed 0 (immediate deadline still `redis:timeout`); `TestIdleTimeoutOpensANewConnection` backdates with the package constant so a stored 0 still dials twice
   → After `Init` and `InitWithOptions` with negative knobs, assert stored `dialTimeout`/`ioTimeout`/`idleTimeout`/`maxIdleConns` equal the package constants
   Status: done
   Argument: `TestInitStoresDefaultTimeouts` asserts Init and negative Options store the package constants (721336d).
