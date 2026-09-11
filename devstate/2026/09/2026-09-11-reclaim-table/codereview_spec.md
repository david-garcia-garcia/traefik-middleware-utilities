# Spec

1. [missing] proposal.md:27 / requirement.md:21 — deliverable `openspec/specs/std_go_reclaim_*` is absent; reclaim specs exist only under `openspec/changes/add-reclaim-table/specs/` and `knowledge/devdocs/std_go_reclaim.md` references paths that are not on disk
   Status: skipped
   Argument: archive copies change specs into `openspec/specs/`; not an implement miss.
2. [missing] design.md:25 / tasks.md:3.2 — compose omits explicit `--experimental.localplugins.reclaimprobe.settings.useunsafe=false` named in the harness decision and task 3.2
   Status: done
   Argument: added the Traefik CLI flag on docker-compose.yml.
