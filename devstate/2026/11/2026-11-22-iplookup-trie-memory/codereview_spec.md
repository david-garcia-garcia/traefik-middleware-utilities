# Spec

1. [wrong] Memory heap metric — `iplookup/zzz_host_map_store_test.go:337` — Decision 6 case (b) requires `runtime.GC` + `HeapAlloc` like the scale-hunt repro; `heapInuseHostMapStore` reads `HeapInuse`
   Fix: Measure the heap delta with `MemStats.HeapAlloc` after `runtime.GC`
   Status: done
   Argument: heapAllocHostMapStore now reads MemStats.HeapAlloc after runtime.GC
