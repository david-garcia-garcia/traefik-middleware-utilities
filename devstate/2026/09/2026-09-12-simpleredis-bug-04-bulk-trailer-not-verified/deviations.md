# Deviations

- [x] taken  keep-alive on `closeAfter` false instead of exporting Accept count
  Asked: export `startRawReplyRedis` Accept count if the two-Accept assertion needs it.
  Instead: leave Accept count unexported; when `closeAfter` is false, `serveRawReply` waits for the client to disconnect so leftover bytes in the pooled reader can be observed.
  Owner: `simpleredis/fake_redis_test.go`
  Why: honouring export-only would not keep the socket writable; remnant proof needs the existing `closeAfter` flag to mean keep-alive. Truncated tests already use `closeAfter: true`.
  By: explore
  Requester: not asked
