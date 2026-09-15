## Purpose

CIDR lookup isolates IPv4 and IPv6 so a prefix written for one address family cannot match the other. Callers pass `net.IP`; the library does not reconstruct client address.

## Requirements

### Requirement: Separate trees per address family
`Helper` SHALL store IPv4 prefixes on one radix tree and IPv6 prefixes on another. `AddCIDR` and `Contains` SHALL choose the tree with `ip.To4() != nil` (IPv4 and IPv4-mapped IPv6 use the IPv4 tree). A prefix SHALL NOT match an address of the other family, including colliding 32-bit patterns.

#### Scenario: IPv4 prefix does not match colliding IPv6
- **WHEN** `1.2.3.4/32` is stored
- **AND** Contains is called with `102:304::1`
- **THEN** found is false

#### Scenario: IPv6 prefix does not match colliding IPv4
- **WHEN** `808:808::/32` is stored
- **AND** Contains is called with `8.8.8.8`
- **THEN** found is false

#### Scenario: Same-family longest prefix still matches
- **WHEN** `1.2.3.4/32` is stored
- **AND** Contains is called with `1.2.3.4`
- **THEN** found is true
- **AND** prefixLen is 32

#### Scenario: IPv4-mapped IPv6 uses the IPv4 tree
- **WHEN** `8.8.8.8/32` is stored
- **AND** Contains is called with `::ffff:8.8.8.8`
- **THEN** found is true
- **AND** prefixLen is 32

### Requirement: Caller owns the looked-up address
Contains SHALL take `net.IP` the caller already selected. The library MUST NOT read HTTP headers, X-Forwarded-For, user, tenant, or Host.

#### Scenario: Lookup does not read HTTP
- **WHEN** Contains is called with an IP
- **THEN** the helper MUST NOT inspect request headers or reconstruct a client address
