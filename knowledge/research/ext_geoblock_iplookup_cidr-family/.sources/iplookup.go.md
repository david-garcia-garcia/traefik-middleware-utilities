---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/7f8e32df0be83b5b4f6879aa9dce775b4c5f7ba6/pkg/iplookup/iplookup.go
title: iplookup.go at PR 86 head
fetched: 2026-09-15
authority: source
ref: david-garcia-garcia/traefik-geoblock@7f8e32df0be83b5b4f6879aa9dce775b4c5f7ba6:pkg/iplookup/iplookup.go
---

IpLookupHelper holds ipv4Tree and ipv6Tree (*ipRadixTree), plus count int.

treeFor(ip net.IP) returns ipv4Tree when ip.To4() != nil, else ipv6Tree.

AddCIDR parses CIDR, inserts into treeFor(block.IP), increments count.

IsContained returns treeFor(ipAddr).contains(ipAddr) as (found, prefixLen, nil).

insert/contains on ipRadixTree still use bitStart 96 for IPv4 and 0 for IPv6 within each tree.
