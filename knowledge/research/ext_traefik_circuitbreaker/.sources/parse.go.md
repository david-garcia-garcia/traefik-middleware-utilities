---
url: https://github.com/vulcand/predicate/blob/18a87524aab1abdcd4b908ec2cebdbc611692fee/parse.go
title: parse.go Parse
fetched: 2026-09-12
authority: source
ref: vulcand/predicate@18a87524aab1abdcd4b908ec2cebdbc611692fee:parse.go
---

Parser.Parse(in) calls go/parser.ParseExpr(in). Empty in is not special-cased; ParseExpr("") returns an error (no operand).
INT literals become int; FLOAT literals become float64 — so LatencyAtQuantileMS(50) vs (50.0) are different types.
