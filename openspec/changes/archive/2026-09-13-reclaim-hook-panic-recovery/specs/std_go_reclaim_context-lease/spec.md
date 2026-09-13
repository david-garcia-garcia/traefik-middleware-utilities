## ADDED Requirements

### Requirement: Create panic or nil create is a create error
If `create` is nil, or panics, the table SHALL treat that the same as `create` returning an error: the key SHALL NOT be stored, every caller waiting on that create SHALL receive an error, and a later `Open` SHALL be free to try again. The table SHALL NOT call Close (nothing was stored). A panic SHALL be wrapped as `fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)`. A nil `create` SHALL be wrapped as `fmt.Errorf("reclaim: create %q: nil create", key)`.

#### Scenario: Create panic unsticks the key
- **WHEN** the first `Open` for a key has a `create` that panics
- **THEN** that `Open` returns an error wrapping the panic
- **AND** the key is not stored
- **AND** a later `Open` for that key with a valid `create` returns a value

#### Scenario: Nil create unsticks the key
- **WHEN** `Open` is called with a nil `create`
- **THEN** `Open` returns an error
- **AND** the key is not stored
- **AND** a later `Open` for that key with a valid `create` returns a value
