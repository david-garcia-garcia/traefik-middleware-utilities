#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Starts Docker Compose, runs Pester Yaegi e2e tests, then tears down.
#>

[CmdletBinding()]
param(
    [switch]$SkipDockerCleanup,
    [switch]$SkipWait,
    [string]$TestPath,
    # One or more suites, e.g. -Suite simpleredis,ratelimit. simpleredis and ratelimit run once per engine.
    [ValidateSet("reclaim", "simpleredis", "ratelimit", "all")]
    [string[]]$Suite = @("all"),
    [ValidateSet("redis", "dragonfly", "valkey")]
    [string]$Engine
)

$ErrorActionPreference = "Stop"

$allEngines = @("redis", "dragonfly", "valkey")
$runAll = $Suite -contains "all"
$runReclaim = $runAll -or ($Suite -contains "reclaim")
# ratelimit runs before simpleredis on each engine: the simpleredis pool-cap case leaves TIME busy-wait scripts running after it
# returns (TIME does not advance inside a script until busy-reply-threshold, about 5s on Redis 7 and Valkey), so the engine
# answers BUSY to whatever runs next.
$engineSuites = @(@("ratelimit", "simpleredis") | Where-Object { $runAll -or ($Suite -contains $_) })
if ($engineSuites.Count -gt 0 -and -not $runAll -and -not $Engine) {
    throw "-Engine redis, dragonfly or valkey is required with -Suite $($engineSuites -join ',')"
}
$engines = @()
if ($Engine) {
    $engines = @($Engine)
}
elseif ($runAll) {
    $engines = $allEngines
}

function Write-Step {
    param([string]$Message)
    Write-Host $Message
}

function Test-RedisHealth {
    param(
        [string]$BackendHost = "redis",
        [string]$ServiceName = "redis",
        [string]$Password = "",
        [int]$TimeoutSeconds = 90,
        [int]$RetryIntervalSeconds = 2
    )

    Write-Step "Waiting for $ServiceName..."
    $elapsed = 0
    do {
        if ($Password -ne "") {
            $pong = docker compose exec -T redis redis-cli -h $BackendHost -a $Password ping 2>$null
        }
        else {
            $pong = docker compose exec -T redis redis-cli -h $BackendHost ping 2>$null
        }
        if ($LASTEXITCODE -eq 0 -and "$pong".Trim() -eq "PONG") {
            Write-Step "$ServiceName is ready"
            return $true
        }
        Start-Sleep $RetryIntervalSeconds
        $elapsed += $RetryIntervalSeconds
    } while ($elapsed -lt $TimeoutSeconds)

    Write-Error "$ServiceName failed to become ready within $TimeoutSeconds seconds"
    return $false
}

function Test-ServiceHealth {
    param(
        [string]$Url,
        [string]$ServiceName,
        [int]$TimeoutSeconds = 90,
        [int]$RetryIntervalSeconds = 2
    )

    Write-Step "Waiting for $ServiceName..."
    $elapsed = 0
    do {
        try {
            $response = Invoke-WebRequest -Uri $Url -Method Get -TimeoutSec 5 -UseBasicParsing
            if ($response.StatusCode -eq 200) {
                Write-Step "$ServiceName is ready"
                return $true
            }
        }
        catch {
            # not ready
        }
        Start-Sleep $RetryIntervalSeconds
        $elapsed += $RetryIntervalSeconds
    } while ($elapsed -lt $TimeoutSeconds)

    Write-Error "$ServiceName failed to become ready within $TimeoutSeconds seconds"
    return $false
}

# Test-EngineStackHealth waits until one engine's Redis, drop, auth, and Traefik /<engine> health respond.
function Test-EngineStackHealth {
    param([string]$Engine)
    $ready = @(
        (Test-RedisHealth -BackendHost $Engine -ServiceName $Engine),
        (Test-RedisHealth -BackendHost "$Engine-drop" -ServiceName "$Engine-drop"),
        (Test-RedisHealth -BackendHost "$Engine-auth" -ServiceName "$Engine-auth" -Password "secret"),
        (Test-ServiceHealth -Url "http://localhost:8000/$Engine" -ServiceName "whoami /$Engine")
    )
    return ($ready -notcontains $false)
}

# Invoke-IntegrationPester runs the given Tests.ps1 files and throws if any It failed.
function Invoke-IntegrationPester {
    param([string[]]$Path)
    $pesterConfig = New-PesterConfiguration
    $pesterConfig.Run.Path = $Path
    $pesterConfig.Output.Verbosity = "Detailed"
    $pesterConfig.Run.Exit = $false
    $pesterConfig.Run.PassThru = $true
    $result = Invoke-Pester -Configuration $pesterConfig
    if ($result.FailedCount -gt 0) {
        throw "$($result.FailedCount) Pester test(s) failed"
    }
}

try {
    Import-Module Pester -Force -ErrorAction Stop

    docker compose version | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose is not available"
    }

    Write-Step "Starting Docker Compose..."
    docker compose up -d
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose up failed"
    }

    if (-not $SkipWait) {
        $ready = @((Test-ServiceHealth -Url "http://localhost:8080/api/rawdata" -ServiceName "Traefik API"))
        if ($runReclaim) {
            $ready += (Test-ServiceHealth -Url "http://localhost:8000/a" -ServiceName "whoami /a")
            $ready += (Test-ServiceHealth -Url "http://localhost:8000/b" -ServiceName "whoami /b")
        }
        foreach ($stackEngine in $engines) {
            $ready += (Test-EngineStackHealth -Engine $stackEngine)
            if ($engineSuites -contains "ratelimit") {
                # Each probe spends one hit on the shared "default" key; the Pester cases use their own keys.
                foreach ($limiter in @("tokenbucket", "windowcounter")) {
                    $ready += (Test-ServiceHealth -Url "http://localhost:8000/$stackEngine-$limiter/a" -ServiceName "whoami /$stackEngine-$limiter/a")
                }
            }
        }
        if ($ready -contains $false) {
            docker logs reclaim-e2e-traefik 2>&1 | Select-Object -Last 80
            throw "services failed to start"
        }
    }

    if ($TestPath) {
        if (-not (Test-Path $TestPath)) {
            throw "test path not found: $TestPath"
        }
        $pesterPath = $TestPath
        $testPathItem = Get-Item $TestPath
        if ($testPathItem.PSIsContainer) {
            $pesterPath = @(Get-ChildItem $TestPath -Filter *.Tests.ps1 | ForEach-Object FullName)
            if ($pesterPath.Count -eq 0) {
                throw "no *.Tests.ps1 under $TestPath"
            }
        }
        if ($Engine) {
            $env:INTEGRATION_ENGINE = $Engine
        }
        Invoke-IntegrationPester -Path $pesterPath
    }
    else {
        if ($runReclaim) {
            Invoke-IntegrationPester -Path "./scripts/integration-tests.reclaim.Tests.ps1"
        }
        foreach ($suiteEngine in $engines) {
            $env:INTEGRATION_ENGINE = $suiteEngine
            foreach ($engineSuite in $engineSuites) {
                Invoke-IntegrationPester -Path "./scripts/integration-tests.$engineSuite.Tests.ps1"
            }
        }
    }
}
finally {
    if (-not $SkipDockerCleanup) {
        docker compose down -v
    }
}
