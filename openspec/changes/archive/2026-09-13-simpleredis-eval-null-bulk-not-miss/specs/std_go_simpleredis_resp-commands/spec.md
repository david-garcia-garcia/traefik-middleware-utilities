## MODIFIED Requirements

### Requirement: Get returns the stored value or a miss
`Get(name)` SHALL send Redis `GET` for that key. A bulk reply SHALL return those bytes. After a one-slot reply, a nil slot (RESP2 null bulk `$-1`) SHALL return an error whose `Error()` text is `redis:miss`. Empty bulk `$0` SHALL return a non-nil empty slice and MUST NOT return `redis:miss`.

#### Scenario: Get hit and miss
- **WHEN** a key has been Set
- **AND** Get is called for that key
- **THEN** Get returns the bytes that were Set
- **WHEN** Get is called for a missing key
- **THEN** Get returns `redis:miss`

#### Scenario: Value with newlines survives
- **WHEN** Set stores a value that contains newline bytes
- **AND** Get is called for that key
- **THEN** Get returns those exact bytes

#### Scenario: Empty bulk is not a miss
- **WHEN** Get receives empty bulk `$0`
- **THEN** Get returns a non-nil empty slice
- **AND** the error is not `redis:miss`

### Requirement: Eval sends EVALSHA then EVAL on NOSCRIPT
`Eval(ctx, script, digest, keys, args)` SHALL have the public signature `Eval(ctx context.Context, script string, digest string, keys []string, args []string) ([][]byte, error)`. Callers SHALL pass the script body and the SHA-1 hex from `ScriptSHA1Hex` (or an equivalent Redis `sha1hex`). `Eval` MUST NOT hash the script body. `Eval` MUST NOT check that `digest` equals `ScriptSHA1Hex(script)`. `Eval` SHALL send Redis `EVALSHA`, the caller `digest`, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. When the error text from that command starts with `NOSCRIPT`, `Eval` SHALL send `EVAL` once with the same script body, `numkeys`, keys, and args. That EVAL is the only place the script body is sent to Redis/Dragonfly so the engine stores it. That `NOSCRIPT` MUST NOT be returned to the caller as the command result. The client MUST NOT send `SCRIPT LOAD` at `Init`. The client MUST NOT export `EvalSha` or `ScriptLoad`. The client MUST NOT keep a digest table or mutex for scripts. The return SHALL keep the same `[][]byte` shape: a `:` integer is one element of decimal digits; a bulk is one element; a top-level null bulk (`$-1`, including Lua `return false`) SHALL be one nil-slot element with a nil error and MUST NOT be `redis:miss`; Eval MUST NOT remap `redis:miss` as a special case of that verb; a Lua or other server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). A Lua indexed table SHALL decode only as a flat array whose elements are bulk strings or integers (or status). Nested tables and `{ err = "..." }` inside an array SHALL return `redis:unsupported-reply`. Scripts that return several values MUST wrap each slot with Lua `tostring` (or return numbers, which become integers). Scripts that touch keys MUST list those keys in `keys` and MUST NOT use `table.maxn`.

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

#### Scenario: Eval null bulk is not a miss
- **WHEN** Eval receives a top-level RESP2 null bulk `$-1` (Lua `return false`)
- **THEN** Eval returns one nil slot
- **AND** the error is not `redis:miss`
