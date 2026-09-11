---
url: https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/server/string_family_test.cc
ref: dragonflydb/dragonfly@36eaa127:src/server/string_family_test.cc
title: StringFamilyTest.Incr
fetched: 2026-09-11
authority: source
---

TEST_F(StringFamilyTest, Incr):
- incrby key1 0 on stored 123456789 → IntArg(123456789)
- incrby key1 0 on stored -123456789 → IntArg(-123456789)
- incrby ne 0 on missing key → IntArg(0)
- incrby key1 1 on value "   -123  " (non-integer) → ErrArg("ERR value is not an integer")
