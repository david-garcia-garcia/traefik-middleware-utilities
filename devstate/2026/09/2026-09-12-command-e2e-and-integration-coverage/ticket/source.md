# Improve command e2e and Traefik integration-test coverage

I want to IMPROVE test coverage. We must ensure that we have tests for all commands in our e2e suite (the one that runs against redis and dragonfly).

Also, because we want to ensure this works flawlessly in Traefik, review test coverage of Integration Tests (the ones that spin up a full Traefik and use a probe middleware to test stuff).

Context for analysis (do not invent extra product asks):
- The e2e suite that talks to live Redis and Dragonfly lives under simpleredis (*_e2e_test.go and related).
- Integration tests that spin up a full Traefik plus a probe middleware live under e2e/ (and similar).
- Bound the ask: close gaps so every public command has e2e coverage against both engines; review Traefik integration-test coverage and close gaps that would miss a command/path used through Traefik. Do not add unrelated product features.
