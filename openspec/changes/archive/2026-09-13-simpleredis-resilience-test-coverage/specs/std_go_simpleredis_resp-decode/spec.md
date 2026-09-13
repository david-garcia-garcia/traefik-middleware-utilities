## ADDED Requirements

### Requirement: Decoder fuzz never panics and never returns values on a dirty stream
The package SHALL include `FuzzReadReply` over the RESP decoder. That target MUST NOT panic. It MUST NOT return values together with a dirty stream. Seed corpus MUST be supplied via `f.Add` and MUST include well-formed status, integer, error, null bulk, empty bulk, bulk, arrays (including nested and null), huge length digits, truncated headers, unknown type bytes, and bulk lengths that do not fit in `int`. The package SHALL also include `FuzzParseLen` that MUST NOT accept a length whose `length+2` overflows. Tests MUST NOT commit a testdata fuzz corpus.

#### Scenario: Seed corpus runs as unit tests
- **WHEN** `go test` runs `FuzzReadReply` without `-fuzz`
- **THEN** each `f.Add` seed is executed
- **AND** none panic
- **AND** none return values while the stream is dirty

#### Scenario: ParseLen fuzz rejects overflowing length
- **WHEN** `FuzzParseLen` is given digit strings including values that would overflow `length+2`
- **THEN** it does not panic
- **AND** it does not report success for an overflowing length
