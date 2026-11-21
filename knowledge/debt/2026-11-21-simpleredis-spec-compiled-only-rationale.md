# Align the `std_go_simpleredis_resp-commands` rationale with "compiled and under Yaegi"

IssueKey: 2026-11-21-ratelimit-benchmarks
Size: large
Action: note

## Why this follow-up
The requirement "RESP framing does not allocate, measured compiled" in `openspec/specs/std_go_simpleredis_resp-commands/spec.md` justifies itself with "the deployment this session is tuned for is compiled into Traefik" and says Yaegi does not make interpreted speed "the optimisation target". The requester stated on 2026-11-21 that the middlewares run both compiled and under Yaegi, and this change corrects the same claim in the README and in `knowledge/devdocs/std_go_simpleredis.md`. After this change the live spec is the only place that still says the deployment is compiled only.

## Why it was not taken
`requirement.md` Desired 5 names only the devdoc sentence. The spec paragraph sits next to normative MUSTs (encoder shape anchored on compiled numbers, the two Yaegi encode benches must stay), and rewording it is a contract edit on a live spec outside this ticket; unattended runs only take small rows on artifacts they created.

## Risks
Readers of the catalog get the old deployment premise and may treat interpreted cost as irrelevant when deciding other SimpleRedis shapes.

## Context
Current: `openspec/specs/std_go_simpleredis_resp-commands/spec.md`, requirement "RESP framing does not allocate, measured compiled", third paragraph.
Proposed: a MODIFIED delta that keeps the encoder rule and the two Yaegi benches but states that the code runs compiled and under Yaegi, and why the compiled numbers still decide the encoder shape.
