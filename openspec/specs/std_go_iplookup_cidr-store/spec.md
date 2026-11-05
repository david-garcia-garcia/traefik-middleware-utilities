## Purpose

Callers insert, remove, and clear CIDR prefixes on a Helper. Count is the number of stored prefixes, not the number of insert calls.

## Requirements

### Requirement: New builds an empty helper
`New()` SHALL return a Helper with Count 0 and no stored prefixes. Construction MUST NOT read HTTP or files.

#### Scenario: Empty helper contains nothing
- **WHEN** New is called
- **AND** Contains is called with `1.2.3.4`
- **THEN** found is false
- **AND** Count is 0

### Requirement: AddCIDR stores a canonical prefix
`AddCIDR(cidr, metadata)` SHALL parse the CIDR, store it on the family tree for that network, and return a parse error when the string is not a CIDR. Two strings that parse to the same network and mask length SHALL be the same stored prefix. Storing a prefix that is already present SHALL replace its metadata and MUST NOT increase Count.

#### Scenario: First insert increments Count
- **WHEN** the helper is empty
- **AND** AddCIDR is called with `10.0.0.0/8` and any metadata
- **THEN** err is nil
- **AND** Count is 1

#### Scenario: Same prefix again does not bump Count
- **WHEN** `10.0.0.0/8` is already stored
- **AND** AddCIDR is called with `10.0.0.1/8` and new metadata
- **THEN** err is nil
- **AND** Count stays 1

#### Scenario: Invalid CIDR is rejected
- **WHEN** AddCIDR is called with `not-a-cidr`
- **THEN** err is not nil
- **AND** Count is unchanged

### Requirement: AddEntry stores a CIDR or one address
`AddEntry(entry, metadata)` SHALL trim surrounding whitespace. Text containing `/` SHALL be stored exactly as `AddCIDR` would store it. Any other text SHALL parse as one address and be stored as `HostCIDR` of that address. An entry that is neither SHALL return an error that names the entry and MUST NOT change Count.

`HostCIDR(ip)` SHALL be `a.b.c.d/32` when `ip.To4()` is non-nil (IPv4 and IPv4-mapped IPv6) and `ip/128` otherwise. An IPv4-mapped literal MUST NOT become an IPv6 `/32`.

#### Scenario: Bare IPv4-mapped address is the IPv4 host
- **WHEN** AddEntry is called with `::ffff:10.10.60.5`
- **THEN** Contains for `10.10.60.5` is found with prefix length 32
- **AND** Contains for `::5` is not found

#### Scenario: Bare IPv6 address is a /128
- **WHEN** AddEntry is called with `2001:db8::1`
- **THEN** Contains for `2001:db8::1` is found with prefix length 128
- **AND** Contains for `2001:db8::2` is not found

#### Scenario: CIDR entry is the AddCIDR prefix
- **WHEN** AddEntry is called with `::ffff:10.0.0.0/104`
- **AND** AddCIDR is called with `::ffff:10.0.0.0/104`
- **THEN** Count is 1

#### Scenario: Invalid entry is rejected
- **WHEN** AddEntry is called with `not-an-ip`
- **THEN** err is not nil and names `not-an-ip`
- **AND** Count is unchanged

### Requirement: RemoveCIDR drops one stored prefix
`RemoveCIDR(cidr)` SHALL parse the CIDR and remove that stored prefix (same canonical network and mask length as AddCIDR). When the prefix is not stored, removed SHALL be false and err SHALL be nil. Parse failure SHALL return an error. Other prefixes, including overlapping different lengths, SHALL stay.

#### Scenario: Remove drops an exact prefix
- **WHEN** `10.0.0.0/8` and `10.1.0.0/16` are stored
- **AND** RemoveCIDR is called with `10.0.0.0/8`
- **THEN** removed is true
- **AND** Count is 1
- **AND** Contains for an address only in `10.0.0.0/8` is false
- **AND** Contains for `10.1.2.3` is still true

#### Scenario: Remove of a missing prefix is not an error
- **WHEN** the helper has no `192.0.2.0/24`
- **AND** RemoveCIDR is called with `192.0.2.0/24`
- **THEN** removed is false
- **AND** err is nil

### Requirement: Reset clears every prefix
`Reset()` SHALL drop all stored prefixes on both family trees. After Reset, Count SHALL be 0 and Contains SHALL return found false for any IP. Reset SHALL be safe to call more than once. Reset is a production clear, not a test-only seam.

#### Scenario: Reset empties both families
- **WHEN** an IPv4 prefix and an IPv6 prefix are stored
- **AND** Reset is called
- **THEN** Count is 0
- **AND** Contains for an IPv4 in the former prefix is false
- **AND** Contains for an IPv6 in the former prefix is false

### Requirement: Mutations are serialized with lookup
AddCIDR, RemoveCIDR, Reset, Contains, and Count SHALL be safe to call concurrently on one Helper.

#### Scenario: Concurrent Contains during Reset
- **WHEN** prefixes are stored
- **AND** Reset and Contains run on other goroutines
- **THEN** neither call panics
- **AND** after Reset returns, a later Contains finds no prefix
