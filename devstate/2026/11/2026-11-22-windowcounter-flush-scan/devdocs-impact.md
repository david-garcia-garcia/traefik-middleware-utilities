# Devdocs impact
change: windowcounter-flush-dirty-index

## Units
- Window counter — subsystem — `windowcounter/limiter.go`

## Findings
- [x] stale-usage  Window counter — `std_go_windowcounter` How-to named `BenchmarkFlush` of 1000 dirty keys and omitted `BenchmarkFlushIdle/{10k,200k}` compiled and Yaegi
