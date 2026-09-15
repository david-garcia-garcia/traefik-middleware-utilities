## Purpose

Each stored CIDR carries a string. Contains returns the string of the winning longest-prefix match.

## ADDED Requirements

### Requirement: AddCIDR stores a match label
`AddCIDR(cidr, metadata)` SHALL associate `metadata` with that prefix. An empty string is a stored value, not an absent label. A later AddCIDR of the same canonical prefix SHALL replace the label.

#### Scenario: Override replaces the label
- **WHEN** `10.0.0.0/8` is stored with metadata `office`
- **AND** AddCIDR is called again with `10.0.0.0/8` and metadata `vpn`
- **AND** Contains is called with `10.1.2.3`
- **THEN** found is true
- **AND** metadata is `vpn`

#### Scenario: Empty label is returned
- **WHEN** `192.0.2.0/24` is stored with metadata `""`
- **AND** Contains is called with `192.0.2.1`
- **THEN** found is true
- **AND** metadata is the empty string

### Requirement: Contains returns the winning prefix label
`Contains(ip)` SHALL return found, the winning prefix length, that prefix's metadata, and a parse/lookup error. When found is false, metadata SHALL be empty and prefixLen SHALL be 0. The winning prefix is the longest stored prefix of the same family that contains the IP. Walked shorter prefixes SHALL NOT be returned.

#### Scenario: Longer prefix wins
- **WHEN** `10.0.0.0/8` is stored with metadata `wide`
- **AND** `10.1.0.0/16` is stored with metadata `narrow`
- **AND** Contains is called with `10.1.2.3`
- **THEN** found is true
- **AND** prefixLen is 16
- **AND** metadata is `narrow`

#### Scenario: Miss returns empty metadata
- **WHEN** no prefix contains `203.0.113.1`
- **AND** Contains is called with that IP
- **THEN** found is false
- **AND** prefixLen is 0
- **AND** metadata is the empty string
