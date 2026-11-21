# Issues

- [ ] note large  `knowledge/debt/2026-11-21-reclaim-waitctx-select.md`
  Why: reclaim `waitCtx` is the last production `select` after this change, and the AST guard covers only simpleredis.
- [ ] note large  `knowledge/debt/2026-11-21-yaegi-tagless-switch-multi-case.md`
  Why: Under Yaegi v0.16.1, a tagless `switch` with a multi-expression `case` sent classified errors to `default` in the SaturatedGet probe. No session source uses that shape today, and the defect has not been isolated.
