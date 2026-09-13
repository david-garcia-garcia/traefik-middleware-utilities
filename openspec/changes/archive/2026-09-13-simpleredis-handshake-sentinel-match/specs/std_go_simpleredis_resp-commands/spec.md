## MODIFIED Requirements

### Requirement: Exported error strings are stable
Callers SHALL match errors by `errors.Is` against the exported sentinel values (`ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrPoolWait`, `ErrUnsupportedReply`) or the predicates `IsMiss`, `IsUnreachable`, and `IsPoolWait`. The session SHALL still export these exact strings for display and legacy text matching: `redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`, `redis:unsupported-reply`. AUTH-class Redis error prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) SHALL map to `redis:noauth`. Other `-` replies SHALL be returned as `errors.New` of that text. `ErrPoolWait` SHALL wrap `ErrUnreachable` so `errors.Is` on the unreachable sentinel matches pool wait, while `IsPoolWait` still distinguishes pool saturation. A wrapped sentinel SHALL still match `errors.Is` and the corresponding predicate; `err.Error() ==` the token MUST NOT be the only supported match. Handshake AUTH or SELECT failures that are those sentinels SHALL match the same way for compiled callers and for Yaegi-interpreted callers. The session MUST NOT return a package-local wrapper whose `Unwrap` compiled `errors.Is` cannot see under Yaegi.

#### Scenario: Rejected auth is redis:noauth
- **WHEN** Redis replies `-NOAUTH`
- **THEN** the command returns `redis:noauth`

#### Scenario: Wrapped miss still matches the miss sentinel
- **WHEN** a miss sentinel is wrapped with `%w`
- **THEN** `errors.Is` and `IsMiss` still match
- **AND** `err.Error()` is not equal to `redis:miss`

#### Scenario: Pool wait matches unreachable and is distinct
- **WHEN** the error is the pool-wait sentinel
- **THEN** `errors.Is` matches the unreachable sentinel
- **AND** `IsPoolWait` is true for pool wait and false for a plain unreachable sentinel

#### Scenario: Unsupported reply is redis:unsupported-reply
- **WHEN** the peer replies with a well-framed type this client does not decode
- **THEN** the command returns `redis:unsupported-reply`
- **AND** the error is not `redis:issue?`

#### Scenario: Handshake AUTH close matches unreachable compiled and interpreted
- **WHEN** a compiled caller issues a command against a peer that accepts TCP, reads AUTH, and closes with no reply
- **THEN** `IsUnreachable` is true
- **WHEN** Yaegi-interpreted code issues that same command against the same kind of peer
- **THEN** `IsUnreachable` is true
- **AND** `Error()` is `redis:unreachable`

#### Scenario: Handshake AUTH WRONGPASS matches ErrNoAuth compiled and interpreted
- **WHEN** a compiled caller issues a command against a fake that replies `-WRONGPASS` to AUTH
- **THEN** `errors.Is` matches `ErrNoAuth`
- **WHEN** Yaegi-interpreted code issues that same command against the same kind of fake
- **THEN** `errors.Is` matches `ErrNoAuth`
- **AND** `Error()` is `redis:noauth`

### Requirement: Interpreter tests observe Init Get Set Del
Tests that import Yaegi v0.16.1 SHALL prove interpreted code can `New`, `Get`, `Set`, `Del`, `Incr`, `Eval`, and `MSetEX` against a compiled fake TCP Redis, including the NOSCRIPT fallback path. Those tests MUST use GOPATH with stdlib symbols only and `useunsafe` false. Those tests MUST NOT start Traefik. Yaegi SHALL cover both MSetEX paths: a fake that implements MSETEX (native), and a fake that rejects MSETEX so the first call falls back to EVAL and a second call does not send `MSETEX`. Interpreted `clientprobe` SHALL also observe `errors.Is` against an exported sentinel and one of `IsMiss`, `IsUnreachable`, or `IsPoolWait`. Those tests SHALL also prove interpreted matchers on errors the package returns: handshake AUTH peer-close (`IsUnreachable`) and handshake AUTH-class (`errors.Is` against `ErrNoAuth`). A `%w` wrap of a sentinel alone MUST NOT be the only interpreted matcher coverage.

#### Scenario: Yaegi Init Get Set Del
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener
- **AND** it Sets a key and Gets that key
- **AND** it Dels that key
- **THEN** Get returns the bytes that were Set
- **AND** a later Get of that key is a miss or the Del returned no error

#### Scenario: Yaegi Incr and Eval
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener
- **AND** it calls Incr on a missing key
- **AND** it calls Eval with a script that returns an integer
- **THEN** Incr returns `1`
- **AND** Eval returns one element whose bytes are that integer

#### Scenario: Yaegi Eval NOSCRIPT fallback
- **WHEN** interpreted code Inits a client to a compiled fake Redis listener whose first EVALSHA for that script is a miss
- **AND** it calls Eval
- **THEN** Eval returns the script result
- **AND** the caller does not see NOSCRIPT

#### Scenario: Yaegi MSetEX native
- **WHEN** interpreted code constructs a client with New to a compiled fake that implements MSETEX
- **AND** it calls MSetEX with one name and value
- **THEN** MSetEX returns no error
- **AND** Get of that name returns the written bytes

#### Scenario: Yaegi MSetEX Lua fallback
- **WHEN** interpreted code constructs a client with New to a compiled fake that replies unknown-command to MSETEX
- **AND** it calls MSetEX twice
- **THEN** the first call returns no error
- **AND** the second call does not send `MSETEX`

#### Scenario: Yaegi matches an exported sentinel
- **WHEN** interpreted code calls `errors.Is` on an exported SimpleRedis sentinel and one predicate
- **THEN** both matches succeed

#### Scenario: Yaegi matches handshake AUTH unreachable
- **WHEN** interpreted code constructs a client with New with a password against a compiled fake that accepts TCP, reads AUTH, and closes with no reply
- **THEN** `IsUnreachable` is true
- **AND** a compiled control against the same kind of peer also matches `IsUnreachable`

#### Scenario: Yaegi matches handshake AUTH noauth
- **WHEN** interpreted code constructs a client with New with a password against a compiled fake that replies `-WRONGPASS` to AUTH
- **THEN** `errors.Is` matches `ErrNoAuth`
- **AND** a compiled control against the same kind of fake also matches `ErrNoAuth`
