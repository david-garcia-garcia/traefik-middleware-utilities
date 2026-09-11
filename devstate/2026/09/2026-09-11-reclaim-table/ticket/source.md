# Spin off reclaim table into traefik-middleware-utilities with Yaegi e2e coverage

We need a fully local OpenDev workflow (no PR, no issue tracker). The purpose is to add to this project the reclaim table package WITH extensive testing, including e2e Pester-based testing that this runs correctly in Yaegi by creating a fake middleware and adding e2e test coverage.

This reclaim table, and everything you need, already exists in this pull request: https://github.com/david-garcia-garcia/traefik-geoblock/pull/83

Clone that branch to access the code in an easier way.

We also need to BRING IN the specs from that project that describe how the reclaim table works, and knowledge related to traefik or yaegi.

We are spinning off that package to its own repo so it can be reused.

This destination repo (README, currently untracked on the caller checkout, not necessarily on origin/initial) already says: shared Go libraries for Traefik middlewares; reclaim table is the first library in scope; layout `reclaim/`; module path `github.com/david-garcia-garcia/traefik-middleware-utilities`; Yaegi constraints listed in README.
