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

### Requirement: IPv4-mapped CIDR insert remaps to IPv4 length
When `AddCIDR` is given a parseable IPv4-mapped CIDR (`To4()` non-nil and mask `bits==128`), insert SHALL remap the prefix to IPv4 length `ones-96` and store it on the IPv4 tree. Insert MUST NOT panic. Insert MUST NOT walk `ones` bits from bit 96 of a 16-byte address. After a successful insert, `Contains` SHALL match `net.IPNet.Contains` for that network: IPv4 and IPv4-mapped addresses that sit in the remapped IPv4 prefix match; native IPv6 does not. Native IPv4 CIDRs (mask `bits==32`) and native IPv6 CIDRs (`To4()` nil) SHALL keep `Mask.Size()`. Mapped prefixes that `ParseCIDR` already rewrites to native IPv6 SHALL stay unchanged. `AddCIDR` MUST NOT fail solely because the CIDR is IPv4-mapped. `RemoveCIDR` of the same CIDR string SHALL drop that remapped prefix.

#### Scenario: Mapped slash-96 does not panic
- **WHEN** AddCIDR is called with `::ffff:0:0/96`
- **THEN** err is nil
- **AND** the call does not panic

#### Scenario: Mapped slash-96 matches IPv4
- **WHEN** `::ffff:0:0/96` is stored
- **AND** Contains is called with `192.0.2.1`
- **THEN** found is true
- **AND** prefixLen is 0

#### Scenario: Mapped slash-96 matches IPv4-mapped
- **WHEN** `::ffff:0:0/96` is stored
- **AND** Contains is called with `::ffff:192.0.2.1`
- **THEN** found is true

#### Scenario: Mapped slash-96 misses native IPv6
- **WHEN** `::ffff:0:0/96` is stored
- **AND** Contains is called with `2001:db8::1` or `::1` or `::`
- **THEN** found is false

#### Scenario: Mapped slash-120 matches as IPv4 slash-24
- **WHEN** `::ffff:192.0.2.0/120` is stored
- **AND** Contains is called with `192.0.2.10`
- **THEN** found is true
- **AND** prefixLen is 24

#### Scenario: Remove of a mapped CIDR drops the remapped prefix
- **WHEN** `::ffff:0:0/96` is stored
- **AND** RemoveCIDR is called with `::ffff:0:0/96`
- **THEN** removed is true
- **AND** Contains for `192.0.2.1` is false

### Requirement: Caller owns the looked-up address
Contains SHALL take `net.IP` the caller already selected. The library MUST NOT read HTTP headers, X-Forwarded-For, user, tenant, or Host.

#### Scenario: Lookup does not read HTTP
- **WHEN** Contains is called with an IP
- **THEN** the helper MUST NOT inspect request headers or reconstruct a client address
