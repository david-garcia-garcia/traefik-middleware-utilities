# Spec

1. [missing] openspec/changes/add-tokenbucket/specs/std_go_tokenbucket_allow/spec.md — Requirement: Unit and Yaegi prove Allow — interpreted tests SHALL run the same Allow scenarios (burst-after-idle and refund); `tokenbucket/yaegi_test.go:15` probes BurstAfterIdle only
   Status: done
   Argument: TestYaegi_AllowRefund + allowprobe.RefundAfterBurst.
