# Standards

1. [hard] Leave a trail — `simpleredis/yaegi_test.go:15` — job comment still names New, Set, Get, and Del after it was left on `TestYaegi_StrayExtraReplyOwnKey`; `TestYaegi_NewGetSetDel` lost the trail
   → Move that comment back onto `TestYaegi_NewGetSetDel`; add a job comment that the new test Gets own-keys against the stray extra fake under Yaegi
   Status: done
   Argument: restored the New/Get/Set/Del comment on `TestYaegi_NewGetSetDel`; new test names the stray-extra Yaegi job.
2. [hard] Leave a trail — `simpleredis/fake_redis_test.go:882` — new `connections` has no job comment; `fakeRedis.connections` and `peerCloseFake.connections` each say it is how many TCP accepts the fake has seen
   → Add the same job comment on this getter
   Status: done
   Argument: added `connections is how many TCP accepts the fake has seen.`
3. [hard] Leave a trail — `simpleredis/resp.go:15` — `do` now refuses a write when `Buffered() != 0` (`errIssue`) and returns the decoded value with `reusable = false` when leftover remains after a complete parse; the job comment only names leftover after a complete value, and neither gate has a block intro
   → Name both leftover jobs on `do`; intro the pre-write refuse and the post-read destroy
   Status: done
   Argument: named both leftover jobs on `do` and intro'd the two gates.
