## ADDED Requirements

### Requirement: ScriptSHA1Hex is Redis sha1hex
The package SHALL export `ScriptSHA1Hex(script string) string`. It SHALL return the SHA-1 of the script bytes as lowercase 40-character hex (Redis `sha1hex`). The client MUST NOT keep a digest table or mutex for scripts. Tests that import Yaegi v0.16.1 SHALL prove interpreted code can call `ScriptSHA1Hex` under GOPATH with stdlib symbols only and `useunsafe` false.

#### Scenario: ScriptSHA1Hex is 40-char lowercase hex
- **WHEN** `ScriptSHA1Hex` is called with a Lua body
- **THEN** the result is 40 lowercase hex characters
- **AND** that value equals SHA-1 of those script bytes

#### Scenario: Yaegi ScriptSHA1Hex
- **WHEN** interpreted code calls `ScriptSHA1Hex` with a script body
- **THEN** the result equals the compiled `ScriptSHA1Hex` of that body

## MODIFIED Requirements

### Requirement: Eval sends EVALSHA then EVAL on NOSCRIPT
`Eval(script, digest, keys, args)` SHALL have the public signature `Eval(script string, digest string, keys []string, args []string) ([][]byte, error)`. Callers SHALL pass the script body and the SHA-1 hex from `ScriptSHA1Hex` (or an equivalent Redis `sha1hex`). `Eval` MUST NOT hash the script body. `Eval` MUST NOT check that `digest` equals `ScriptSHA1Hex(script)`. `Eval` SHALL send Redis `EVALSHA`, the caller `digest`, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. When the error text from that command starts with `NOSCRIPT`, `Eval` SHALL send `EVAL` once with the same script body, `numkeys`, keys, and args. That EVAL is the only place the script body is sent to Redis/Dragonfly so the engine stores it. That `NOSCRIPT` MUST NOT be returned to the caller as the command result. The client MUST NOT send `SCRIPT LOAD` at `Init`. The client MUST NOT export `EvalSha` or `ScriptLoad`. The client MUST NOT keep a digest table or mutex for scripts. The return SHALL keep the same `[][]byte` shape: a `:` integer is one element of decimal digits; a bulk is one element; a Lua or other server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). A Lua indexed table SHALL decode only as a flat array whose elements are bulk strings or integers (or status). Nested tables and `{ err = "..." }` inside an array SHALL return `redis:unsupported-reply`. Scripts that return several values MUST wrap each slot with Lua `tostring` (or return numbers, which become integers). Scripts that touch keys MUST list those keys in `keys` and MUST NOT use `table.maxn`.

#### Scenario: Later Eval sends EVALSHA not the body
- **WHEN** Eval is called twice with the same script, that script’s `ScriptSHA1Hex` digest, one key, and two args against a fake that already has that digest
- **THEN** the second command sent is `EVALSHA`, that digest, `1`, that key, then those args
- **AND** the second argv MUST NOT include the script body

#### Scenario: First EVALSHA miss falls back to EVAL
- **WHEN** the first `EVALSHA` for a script receives `-NOSCRIPT No matching script. Please use EVAL.`
- **THEN** Eval sends `EVAL`, that script body, the same `numkeys`, keys, and args
- **AND** Eval returns the EVAL result
- **AND** the caller error is not `NOSCRIPT`

#### Scenario: After EVAL the digest hits
- **WHEN** EVAL has loaded that digest on the fake
- **AND** Eval is called again with the same script and that digest
- **THEN** the command sent is `EVALSHA` with that digest
- **AND** Eval succeeds without a second EVAL

#### Scenario: Two scripts two digests
- **WHEN** Eval is called with script A and its digest then with a different script B and its digest
- **THEN** the `EVALSHA` argv digests differ

#### Scenario: Eval uses the caller digest
- **WHEN** Eval is called with a script and a digest equal to `ScriptSHA1Hex` of that script
- **THEN** the `EVALSHA` argv digest is that caller string
- **AND** Eval MUST NOT replace it with a newly hashed value

#### Scenario: Eval integer reply
- **WHEN** Redis replies to EVALSHA or EVAL with a `:` integer
- **THEN** Eval returns one `[][]byte` element whose bytes are that decimal payload

#### Scenario: Eval empty keys
- **WHEN** Eval is called with a script, its digest, no keys, and no args
- **THEN** the command sent includes `numkeys` `0`

#### Scenario: Eval three bulk strings
- **WHEN** Redis replies to Eval with a three-element array of bulk strings
- **THEN** Eval returns those three payloads in order

#### Scenario: Eval nested array is unsupported
- **WHEN** Redis replies to Eval with a nested array
- **THEN** Eval returns `redis:unsupported-reply`
- **AND** the idle pool is empty
- **AND** the next command on that client dials a new socket
