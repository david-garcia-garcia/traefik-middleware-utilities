# Deviations

- [x] taken  dest bulk-length `$27` instead of the ask's `$28` example
  Asked: encoded GET `session:9f2c1ab4-user-token` equals `*2\r\n$3\r\nGET\r\n$28\r\nsession:9f2c1ab4-user-token\r\n`.
  Instead: the same argv framed as dest `writeCommand` would (`$27`, the key's byte length).
  Owner: `simpleredis/resp.go` `appendRESP`
  Why: honouring `$28` would emit a wrong bulk length; the job is identical dest wire bytes, and `$28` was a count error in the example.
  By: implement
  Requester: not asked
