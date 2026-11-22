# Path-compress the iplookup prefix trie for large non-host CIDR lists

IssueKey: 2026-11-22-iplookup-trie-memory
Size: large
Action: note

## Why this follow-up

Host /32 and /128 lists now sit in pointer-free maps. Any prefix shorter than the family maximum still uses `ipRadixTree`: one `radixNode` per bit that is not shared. A large list of ranges (geofeeds, ISP blocks, `10.0.0.0/8` style allowlists at high N) keeps the old heap and GC-pointer cost this change removed for hosts.

## Why it was not taken

This change's binding design is host maps plus the existing trie. Compressing the trie (Patricia / path compression) is a separate representation with its own Yaegi and longest-prefix risks, and the live contracts do not require it.

## Risks

A feed that is mostly non-host prefixes still allocates tens to hundreds of bytes per prefix and a pointer graph the GC must mark. Reload via `Reset` plus rebuild still doubles that peak until the next GC.
