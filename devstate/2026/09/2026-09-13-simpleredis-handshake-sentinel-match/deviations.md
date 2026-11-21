# Deviations

- [x] taken  bool named `handshakeFailed` instead of the ask's `handshake`
  Asked: `dial` returns `(*pooledConn, error, handshake bool)`.
  Instead: the same triple return with the bool named `handshakeFailed` (true only after TCP succeeded and AUTH/SELECT failed).
  Owner: `simpleredis/pool.go`
  Why: a bare `handshake` bool does not name the cases `shouldRetry` cares about; honouring the ask's identifier would leave a complement-shaped flag on the retry path.
  By: explore
  Requester: not asked
