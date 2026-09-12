#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Starts Docker Compose, runs Pester Yaegi e2e tests, then tears down.
#>

[CmdletBinding()]
param(
    [switch]$SkipDockerCleanup,
    [switch]$SkipWait,
    [string]$TestPath = "./scripts"
)

$ErrorActionPreference = "Stop"

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
        $ready = @(
            (Test-ServiceHealth -Url "http://localhost:8080/api/rawdata" -ServiceName "Traefik API"),
            (Test-RedisHealth -BackendHost "redis" -ServiceName "redis"),
            (Test-RedisHealth -BackendHost "dragonfly" -ServiceName "dragonfly"),
            (Test-RedisHealth -BackendHost "redis-drop" -ServiceName "redis-drop"),
            (Test-RedisHealth -BackendHost "dragonfly-drop" -ServiceName "dragonfly-drop"),
            (Test-RedisHealth -BackendHost "redis-auth" -ServiceName "redis-auth" -Password "secret"),
            (Test-RedisHealth -BackendHost "dragonfly-auth" -ServiceName "dragonfly-auth" -Password "secret"),
            (Test-ServiceHealth -Url "http://localhost:8000/a" -ServiceName "whoami /a"),
            (Test-ServiceHealth -Url "http://localhost:8000/b" -ServiceName "whoami /b"),
            (Test-ServiceHealth -Url "http://localhost:8000/redis" -ServiceName "whoami /redis"),
            (Test-ServiceHealth -Url "http://localhost:8000/dragonfly" -ServiceName "whoami /dragonfly")
        )
        if ($ready -contains $false) {
            docker logs reclaim-e2e-traefik 2>&1 | Select-Object -Last 80
            throw "services failed to start"
        }
    }

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

    $pesterConfig = New-PesterConfiguration
    $pesterConfig.Run.Path = $pesterPath
    $pesterConfig.Output.Verbosity = "Detailed"
    $pesterConfig.Run.Exit = $false
    $pesterConfig.Run.PassThru = $true
    $result = Invoke-Pester -Configuration $pesterConfig

    if ($result.FailedCount -gt 0) {
        throw "$($result.FailedCount) Pester test(s) failed"
    }
}
finally {
    if (-not $SkipDockerCleanup) {
        docker compose down -v
    }
}
