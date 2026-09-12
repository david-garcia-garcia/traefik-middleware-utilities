# Traefik compose URLs and log wait for Pester. Dot-sourced via Import.ps1.

# Get-IntegrationBaseUrl is the Traefik entry URL in compose.
function Get-IntegrationBaseUrl {
    "http://localhost:8000"
}

# Get-TraefikApiUrl is the Traefik API URL in compose.
function Get-TraefikApiUrl {
    "http://localhost:8080"
}

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
    $response = Invoke-WebRequest -Uri "$(Get-TraefikApiUrl)/api/rawdata" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
    $response.StatusCode | Should -Be 200 -Because $response.Content
}
