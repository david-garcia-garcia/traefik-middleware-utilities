## 1. Usage gotcha

- [ ] 1.1 Add the gotcha bullet from design.md decision 4 to `knowledge/devdocs/std_go_simpleredis.md` `## Gotchas`, next to the existing retry / `IOTimeout` bullets. Do not restyle neighboring bullets.
- [ ] 1.2 Leave `defaultIOTimeout` and every `simpleredis/*.go` file untouched.

## 2. Spec delta

- [ ] 2.1 Keep the ADDED requirement `Timeout second-connection rule is per command` on this change's `specs/std_go_simpleredis_tcp-session/spec.md`. Do not rewrite the existing I/O deadline requirement in the main spec until archive.
- [ ] 2.2 `openspec validate --change simpleredis-timeout-connect-per-command --strict`

## 3. Guards

- [ ] 3.1 Confirm `git diff origin/master -- simpleredis/` is empty.
- [ ] 3.2 `go test ./simpleredis/ -count=1 -short` still passes (no new test; sanity only).
