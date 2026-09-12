# Issues

- [ ] note large  `knowledge/debt/2026-09-12-dial-handshake-panic-leaks-token.md`
  Why: `dial` AUTH/SELECT call `do` while still holding the in-use-turn; a panic there still drops the token.
