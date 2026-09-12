# Nitpicks

1. [hard] Symmetry and consistency — `Test-Integration.ps1:174` — the `-Engine` parameter and `Test-EngineStackHealth -Engine $Engine` spell the integration backend `Engine`, but the default-suite loop uses a different identifier for the same role:

```powershell
foreach ($name in $engines) {
    $env:INTEGRATION_ENGINE = $name
    Invoke-IntegrationPester -Path "./scripts/integration-tests.simpleredis.Tests.ps1"
}
```

   → Rename the loop variable to `$engine` (or `$Engine` if shadowing is acceptable in this block) so sibling paths use the same name for the redis/dragonfly selector.
   Status: done
   Argument: loop variable is `$Engine`.

2. [hard] Name for the scope — `scripts/integration-tests.utils/simpleredis.ps1:158` — health-token bodies are stored as single-letter placeholders:

```powershell
$a = $first.Content.Trim()
$b = $second.Content.Trim()
$a | Should -Match '^srp:\d+$'
$b | Should -Match '^srp:\d+$'
$a | Should -Not -Be $b
```

   → Name each value for its role in this body (e.g. `$firstToken` / `$secondToken`).
   Status: done
   Argument: `$firstToken` / `$secondToken`.

3. [hard] Name for the scope — `scripts/integration-tests.utils/simpleredis.ps1:70` — query assembly iterates Redis key names as `$name` while the mandatory engine parameter on the same function is `$Engine`:

```powershell
foreach ($name in @($Key)) {
    if ($null -ne $name -and $name -ne "") {
        $parts += "key=$([uri]::EscapeDataString($name))"
    }
}
```

   → Use `$key` (or `$redisKey`) in the loop so the local names the value’s role and does not reuse a generic `$name` beside `$Engine`.
   Status: done
   Argument: loop variable is `$redisKey` (PowerShell `$key` would alias `$Key`).
