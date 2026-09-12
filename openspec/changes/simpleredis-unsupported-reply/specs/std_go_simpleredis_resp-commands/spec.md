## MODIFIED Requirements

### Requirement: Exported error strings are stable
Callers SHALL match errors by `Error()` text. The session SHALL export these exact strings: `redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`, `redis:unsupported-reply`. AUTH-class Redis error prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. Other `-` replies SHALL be returned as `errors.New` of that text.

#### Scenario: Rejected auth is redis:noauth
- **WHEN** Redis replies `-NOAUTH`
- **THEN** the command returns `redis:noauth`

#### Scenario: Unsupported reply is redis:unsupported-reply
- **WHEN** the peer replies with a well-framed type this client does not decode
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the error is not `redis:issue?`

### Requirement: Array replies accept bulk integer and status elements
An RESP array (`*`) SHALL accept each element whose head is `$` (bulk, including null bulk as a nil slot), `:` (integer payload bytes), or `+` (status payload bytes). If an element head is `*` or `-`, the client SHALL return `redis:unsupported-reply`. Nested arrays are out of scope. MGET callers MUST still observe only bulk slots from Redis MGET. A `:` or `+` slot SHALL be an independent copy of those payload bytes so a later read on the same connection cannot overwrite it.

#### Scenario: Mixed array elements
- **WHEN** Redis replies with an array that contains a bulk, an integer, and a status
- **THEN** the result has three slots with those payloads
- **WHEN** an array element is a nested array
- **THEN** the client returns `redis:unsupported-reply`

#### Scenario: Integer and status slots survive a later read
- **WHEN** an array reply stores a `:` payload and a `+` payload
- **AND** a later command is read on the same connection
- **THEN** those stored slots still equal the original payloads

### Requirement: Eval sends EVALSHA then EVAL on NOSCRIPT
`Eval(script, keys, args)` SHALL keep the public signature `Eval(script string, keys []string, args []string) ([][]byte, error)`. Callers pass the script body; they MUST NOT pass a digest. `Eval` SHALL hash the script body on each call (SHA-1 lowercase hex; cheap; no map, no lock) and send Redis `EVALSHA`, that digest, the decimal `numkeys` equal to `len(keys)`, then each key, then each arg. Empty `keys` and empty `args` are legal. When the error text from that command starts with `NOSCRIPT`, `Eval` SHALL send `EVAL` once with the same script body, `numkeys`, keys, and args. That EVAL is the only place the script body is sent to Redis/Dragonfly so the engine stores it. That `NOSCRIPT` MUST NOT be returned to the caller as the command result. The client MUST NOT send `SCRIPT LOAD` at `Init`. The client MUST NOT export `EvalSha` or `ScriptLoad`. The client MUST NOT keep a digest table or mutex for scripts. The return SHALL keep the same `[][]byte` shape: a `:` integer is one element of decimal digits; a bulk is one element; a Lua or other server `-` error SHALL be returned as an error (AUTH-class prefixes still `redis:noauth`). A Lua indexed table SHALL decode only as a flat array whose elements are bulk strings or integers (or status). Nested tables and `{ err = "..." }` inside an array SHALL return `redis:unsupported-reply`. Scripts that return several values MUST wrap each slot with Lua `tostring` (or return numbers, which become integers). Scripts that touch keys MUST list those keys in `keys` and MUST NOT use `table.maxn`.

#### Scenario: Later Eval sends EVALSHA not the body
- **WHEN** Eval is called twice with the same script, one key, and two args against a fake that already has that digest
- **THEN** the second command sent is `EVALSHA`, the SHA-1 hex of that script, `1`, that key, then those args
- **AND** the second argv MUST NOT include the script body

#### Scenario: First EVALSHA miss falls back to EVAL
- **WHEN** the first `EVALSHA` for a script receives `-NOSCRIPT No matching script. Please use EVAL.`
- **THEN** Eval sends `EVAL`, that script body, the same `numkeys`, keys, and args
- **AND** Eval returns the EVAL result
- **AND** the caller error is not `NOSCRIPT`

#### Scenario: After EVAL the digest hits
- **WHEN** EVAL has loaded that digest on the fake
- **AND** Eval is called again with the same script
- **THEN** the command sent is `EVALSHA` with that digest
- **AND** Eval succeeds without a second EVAL

#### Scenario: Two scripts two digests
- **WHEN** Eval is called with script A then with a different script B
- **THEN** the `EVALSHA` argv digests differ

#### Scenario: Eval integer reply
- **WHEN** Redis replies to EVALSHA or EVAL with a `:` integer
- **THEN** Eval returns one `[][]byte` element whose bytes are that decimal payload

#### Scenario: Eval empty keys
- **WHEN** Eval is called with a script, no keys, and no args
- **THEN** the command sent includes `numkeys` `0`

#### Scenario: Eval three bulk strings
- **WHEN** Redis replies to Eval with a three-element array of bulk strings
- **THEN** Eval returns those three payloads in order

#### Scenario: Eval nested array is unsupported
- **WHEN** Redis replies to Eval with a nested array
- **THEN** Eval returns `redis:unsupported-reply`
- **AND** the idle pool is empty
- **AND** the next command on that client dials a new socket

### Requirement: Malformed RESP is a protocol issue and is not pooled
A line that does not end in CR before LF, an empty line, an unparseable `*` count, an `*` count less than 0, or a truncated array element or bulk SHALL return an error whose `Error()` text is `redis:issue?`, or an I/O error (`redis:unreachable` on EOF, `redis:timeout` on deadline). That connection MUST NOT re-enter the idle pool. Unknown type bytes, nested arrays, and array elements whose type is not `$`, `:`, or `+` are `redis:unsupported-reply` (requirement Unsupported RESP replies are distinguishable and are not pooled), not `redis:issue?`.

#### Scenario: Missing CR before LF
- **WHEN** a reply line ends in LF without a preceding CR
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Empty line
- **WHEN** the peer replies with a CRLF-only line
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Unparseable array count
- **WHEN** the peer replies with `*` followed by a payload that is not a signed integer
- **THEN** the command returns `redis:issue?`
- **AND** the idle pool is empty

#### Scenario: Null array is not a miss
- **WHEN** the peer replies with RESP2 null array `*-1`
- **THEN** the command returns `redis:issue?`
- **AND** the error is not `redis:miss`
- **AND** the idle pool is empty

#### Scenario: Truncated array or bulk is I/O
- **WHEN** the peer writes a partial array or bulk and closes the socket
- **THEN** the command returns `redis:unreachable`
- **AND** the idle pool is empty

#### Scenario: Truncated bulk after a complete ReadSlice head returns own-value on the next Get
- **WHEN** `MaxRetries` is `-1`
- **AND** a Get receives `$100\r\n`, then 40 bytes, then a close
- **THEN** that Get returns `redis:unreachable`
- **AND** the idle pool is empty
- **WHEN** a later Get receives a complete bulk of known bytes
- **THEN** that Get returns those bytes

#### Scenario: Retry after a dirty reused connection cannot dial
- **WHEN** a reused idle connection is dirtied by a truncated reply
- **AND** the retry dial fails
- **THEN** the command returns `redis:unreachable`
- **AND** the idle pool is empty

## ADDED Requirements

### Requirement: Unsupported RESP replies are distinguishable and are not pooled
A well-framed reply whose type byte is not `+`, `-`, `:`, `$`, or `*` (including an HTTP-shaped first line and RESP3 type bytes `_`, `#`, `,`, `(`, `%`, `~`, `=`, `>`), or an array element whose type is not `$`, `:`, or `+` (including a nested array or a `-` error inside an array), SHALL return an error whose `Error()` text is `redis:unsupported-reply`. That error MUST NOT be `redis:issue?` and MUST NOT be `redis:unreachable` solely because the type was unsupported. That connection MUST NOT re-enter the idle pool. A following command on the same client SHALL dial a new socket.

#### Scenario: Unknown type including HTTP-shaped
- **WHEN** the peer replies with a line whose first byte is not `+`, `-`, `:`, `$`, or `*`
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: RESP3 type byte
- **WHEN** the peer replies with a RESP3 type byte `_`, `#`, `,`, `(`, `%`, `~`, `=`, or `>`
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: Bad array element type
- **WHEN** an array element’s type byte is not `$`, `:`, or `+`
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: Nested array is not pooled
- **WHEN** an array element is a nested array
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty

#### Scenario: Error inside an array
- **WHEN** an array element is a Redis error line
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the idle pool is empty
