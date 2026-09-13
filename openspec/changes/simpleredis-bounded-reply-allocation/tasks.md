## 1. Decoder

- [ ] 1.1 In `readBulk`, after the over-cap check, read `length+2` with chunked `io.ReadFull`; first `make` is `min(length+2, 128 << 10)`; grow by that chunk. Do not change deadline handling
- [ ] 1.2 After a complete fill, keep the CRLF trailer check and return `data[:length]`
- [ ] 1.3 In `readReply` `*`, after the over-cap check, `make([][]byte, 0, min(count, 16))` and append as elements decode. Keep miss / unsupported-element sentinels

## 2. Tests

- [ ] 2.1 Untagged `readReply` `TotalAlloc` proof: `$` + `maxBulkLength` and `*` + `maxArrayCount`, each with no payload, MUST NOT grow by the announced size. Prefix helpers `allocAmp`
- [ ] 2.2 Keep over-cap, trailer, fuzz, and dest decode alloc ceilings green
- [ ] 2.3 Run `go vet ./simpleredis/`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`, fuzz seeds, `go test -bench . ./simpleredis/`, and `go test -tags simpleredis_bugs ./simpleredis/ -run TestBugPeerControlledAllocation`

## 3. Specs

- [ ] 3.1 Confirm delta `std_go_simpleredis_resp-decode` matches the landed code
- [ ] 3.2 Run `openspec validate --change simpleredis-bounded-reply-allocation --strict`
