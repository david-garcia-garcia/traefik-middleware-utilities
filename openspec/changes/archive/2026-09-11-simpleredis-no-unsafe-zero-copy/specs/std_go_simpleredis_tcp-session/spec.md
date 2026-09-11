## ADDED Requirements

### Requirement: Session source keeps copy conversions
The SimpleRedis session source SHALL convert command names, scripts, and decimal arguments with `[]byte(...)` and integer-reply payloads with `string(...)`. It MUST NOT add `unsafe` zero-copy helpers in session source. It MUST NOT import `unsafe` or use cgo. Traefik local-plugin `useunsafe` MUST stay false.

#### Scenario: Copy conversions stay in session source
- **WHEN** the session source converts a string key, script, or decimal argument to bytes, or a bulk integer payload to a string
- **THEN** that conversion is `[]byte(...)` or `string(...)`
- **AND** session source has no unsafe pointer or header cast that aliases string and `[]byte`
