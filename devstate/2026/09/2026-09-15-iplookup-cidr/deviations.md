# Deviations

- [x] taken  house type and method names instead of geoblock `IpLookupHelper` / `IsContained` / `NewEmptyIpLookupHelper`
  Asked: port the PR #86 helper under those geoblock identifiers.
  Instead: `iplookup.Helper`, `New()`, `Contains`, `AddCIDR` / `RemoveCIDR` / `Reset` / `Count`.
  Owner: `iplookup/`
  Why: this module's packages use `New` plus a short type (`reclaim.Table`, `backendbackoff.Gate`); repeating geoblock's type name would add a second naming shape for the same job.
  By: propose
  Requester: not asked
