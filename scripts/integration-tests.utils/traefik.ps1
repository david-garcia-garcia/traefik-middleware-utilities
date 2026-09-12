# Traefik compose URLs and log wait for Pester. Dot-sourced via Import.ps1.

$env:INTEGRATION_BASE_URL = "http://localhost:8000"
$env:INTEGRATION_TRAEFIK_API_URL = "http://localhost:8080"

# Wait-TraefikPluginLog is true when docker logs reclaim-e2e-traefik match Pattern before TimeoutSeconds.
function Wait-TraefikPluginLog {
    param(
        [string]$Pattern,
        [int]$TimeoutSeconds = 60,
        [int]$RetrySeconds = 2
    )
    $elapsed = 0
    do {
        $logs = docker logs reclaim-e2e-traefik 2>&1 | Out-String
        if ($logs -match $Pattern) {
            return $true
        }
        Start-Sleep $RetrySeconds
        $elapsed += $RetrySeconds
    } while ($elapsed -lt $TimeoutSeconds)
    return $false
}

# Assert-TraefikApiReachable is HTTP 200 on the Traefik API rawdata URL.
function Assert-TraefikApiReachable {
    $response = Invoke-WebRequest -Uri "$env:INTEGRATION_TRAEFIK_API_URL/api/rawdata" -UseBasicParsing -TimeoutSec 10
    $response.StatusCode | Should -Be 200
}
