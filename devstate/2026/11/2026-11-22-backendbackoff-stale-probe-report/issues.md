# Issues

- [ ] note small  README Backend backoff snippet still calls `Allow(req.Context(), …)`
  Why: live spec and usage packet are context-free since nocontext; this ticket's API change will touch the same snippet. Not taken here.
