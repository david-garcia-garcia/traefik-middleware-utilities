# Standards

1. [hard] One job, one owner — `iplookup/store.go:24` — `labelTable` is a second live unit (intern table) in the family-store file
   Fix: Move `labelTable` plus `newLabelTable` / `intern` / `unref` to `iplookup/labeltable.go`
   Status: done
   Argument: moved labelTable plus intern/unref to iplookup/labeltable.go
   Quote:
   ```
   type ipv4Store struct {
   	hosts map[uint32]uint32
   	tree  *ipRadixTree
   }

   type ipv6Store struct {
   	hosts map[[16]byte]uint32
   	tree  *ipRadixTree
   }

   // labelTable interns match labels so host maps hold ids instead of strings.
   type labelTable struct {
   	id   map[string]uint32
   	text []string
   	refs []uint32
   	free []uint32
   }
   ```
2. [hard] Name for the scope — `iplookup/store.go:27` — field `id` hides the intern index (label text → intern id)
   Fix: Rename `labelTable.id` to `byLabel`
   Status: done
   Argument: renamed labelTable.id to byLabel
   Quote:
   ```
   type labelTable struct {
   	id   map[string]uint32
   	text []string
   	refs []uint32
   	free []uint32
   }
   ```
3. [hard] Consume before produce — `iplookup/zzz_host_map_store_test.go:239` — `coversHostMapStore` re-walks prefix bits instead of `net.IPNet.Contains`
   Fix: Family-check, then `cidr.Contains(ip)`, then `prefixWalk` only for the remapped length
   Status: done
   Argument: coversHostMapStore uses family check, net.IPNet.Contains, then prefixWalk for remapped length
   Quote:
   ```
   func coversHostMapStore(cidr *net.IPNet, ip net.IP) (covers bool, prefixLen int) {
   	if (cidr.IP.To4() != nil) != (ip.To4() != nil) {
   		return false, 0
   	}
   	walkIP, maxLen := familyWalk(ip)
   	cidrWalk, storedLen := prefixWalk(cidr)
   	if storedLen > maxLen {
   		return false, 0
   	}
   	for i := 0; i < storedLen; i++ {
   		if bitAt(cidrWalk, i) != bitAt(walkIP, i) {
   			return false, 0
   		}
   	}
   	return true, storedLen
   }
   ```
4. [hard] Leave a trail — `iplookup/zzz_bench_yaegi_test.go:88` — GOPATH harness helpers have no job comment (`zzz_helper_yaegi_test.go` comments the same jobs)
   Fix: Add a one-line job comment on `runYaegiHostMapStoreBench`, `writeGopathIplookupHostMapStore`, `writeGopathFileHostMapStore`, and `callerDirHostMapStore`
   Status: done
   Argument: one-line job comments on the four GOPATH harness helpers
   Quote:
   ```
   func writeGopathIplookupHostMapStore(b *testing.B, goPath string) {
   	b.Helper()
   	srcDir := callerDirHostMapStore(b)
   ```
