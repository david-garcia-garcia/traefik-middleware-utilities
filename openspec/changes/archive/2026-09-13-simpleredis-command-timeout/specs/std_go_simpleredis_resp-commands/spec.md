## MODIFIED Requirements

### Requirement: Eval and MSetEX fallback hops each have a full command budget
`Eval` SHALL send EVALSHA then, on a `NOSCRIPT` prefix, EVAL as two separate command executions. Each hop SHALL compute its own overall deadline at that hop’s entry equal to `now + CommandTimeout` unless the caller’s context deadline is sooner. The EVAL hop MUST NOT inherit remaining time from the EVALSHA hop. Sharing remaining time across those hops can starve EVAL after a slow EVALSHA.

`MSetEX` and `MSetEXAt` SHALL send native MSETEX as its own command execution. On `ERR unknown command`, the Lua fallback via `Eval` SHALL start with a full command budget (one or two further hops). Comments on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex` SHALL record that each hop binds its own overall deadline on purpose.

Compiled tests SHALL prove Eval NOSCRIPT fallback still succeeds and that MSetEX unknown-command fallback still succeeds. Those tests MUST NOT fail because elapsed time exceeds one public-command budget. This change MUST NOT bind one stacked deadline across hops and MUST NOT change `exec` deadline binding.

#### Scenario: Eval NOSCRIPT fallback still succeeds
- **WHEN** the first EVALSHA for a script receives `-NOSCRIPT No matching script. Please use EVAL.`
- **THEN** Eval sends EVAL with that script body, the same `numkeys`, keys, and args
- **AND** Eval returns the EVAL result
- **AND** the caller error is not `NOSCRIPT`

#### Scenario: MSetEX unknown-command fallback still succeeds
- **WHEN** the first native MSETEX receives `-ERR unknown command`
- **THEN** MSetEX runs the Lua fallback via Eval
- **AND** MSetEX returns no error
- **AND** the caller error is not unknown-command
