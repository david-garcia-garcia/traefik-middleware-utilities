## ADDED Requirements

### Requirement: Interpreter tests assert the unsafe conversion matrix
Tests that import Yaegi SHALL assert which `string`/`[]byte` conversions the interpreter accepts under stdlib-only symbols, stdlib plus unsafe symbols, and unrestricted. Those tests MAY register Yaegi unsafe symbols and MAY import `unsafe` in `_test.go` files. Existing Init/Get/Set/Del/Incr/Eval interpreter tests MUST still use GOPATH with stdlib symbols only and `useunsafe` false. Named copy-versus-unsafe benches SHALL exist so a human can reproduce the measured ns/op; they MUST NOT fail `go test` without `-bench`. Those tests MUST NOT start Traefik.

#### Scenario: Matrix cells match the measured table
- **WHEN** the unsafe-variant interpreter test runs under stdlib only, stdlib plus unsafe symbols, and unrestricted
- **THEN** go-redis v9 `unsafe.Slice` / `unsafe.String` is unsupported in every mode
- **AND** the legacy pointer-cast, struct-header, and `reflect.StringHeader` conversions are unsupported under stdlib only and supported when unsafe symbols are registered
- **AND** a cell mismatch fails the test

#### Scenario: Named copy versus unsafe benches exist
- **WHEN** a human runs the named compiled Eval-encode, parse-int, and interpreted convert benches
- **THEN** those benches measure copy versus unsafe conversions
- **AND** `go test` without `-bench` still passes

### Requirement: Compiled tests reject production unsafe
Compiled tests SHALL fail when a non-test file in the SimpleRedis session folder imports `unsafe` or `"C"`, or a non-stdlib dotted path. Compiled tests SHALL fail when the SimpleRedis probe plugin manifest or compose `simpleredisprobe` `useUnsafe` is true. Absent or false on the manifest SHALL pass. Those tests MUST NOT require an explicit `useUnsafe: false` on the manifest. Those tests MUST NOT scan the reclaim probe. Redis and Dragonfly Pester proofs of existing verbs MUST stay. Eval scripts that touch keys MUST list those keys (Dragonfly). Lua MUST stay 5.1-safe. EVALSHA MUST NOT be added.

#### Scenario: Session source import scan
- **WHEN** compiled tests list imports of non-test files in the SimpleRedis session folder
- **THEN** the test fails if any import path is `unsafe` or `"C"` or contains a dot
- **AND** `_test.go` files MAY import `unsafe`

#### Scenario: Probe useUnsafe scan
- **WHEN** compiled tests read the SimpleRedis probe Traefik manifest and the compose `simpleredisprobe` `useunsafe` setting
- **THEN** the test passes if the manifest field is absent or false and compose is false
- **AND** the test fails if either is true
- **AND** the reclaim probe is not scanned
