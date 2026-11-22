# Rate-limit probe helpers for Pester. Dot-sourced via Import.ps1.
# Engine comes from INTEGRATION_ENGINE (set by Test-Integration.ps1 -Engine).

# New-RateLimitKey is a unique X-Probe-Key so each It owns its budget.
function New-RateLimitKey {
    return "rl-$([DateTime]::UtcNow.Ticks)-$([guid]::NewGuid().ToString('N').Substring(0, 8))"
}

# Invoke-RateLimitProbe GETs /<Engine>-<Limiter>/<Instance> with X-Probe-Key Key.
function Invoke-RateLimitProbe {
    param(
        [Parameter(Mandatory)]
        [string]$Engine,
        [Parameter(Mandatory)]
        [ValidateSet("tokenbucket", "windowcounter")]
        [string]$Limiter,
        [Parameter(Mandatory)]
        [ValidateSet("a", "b")]
        [string]$Instance,
        [Parameter(Mandatory)]
        [string]$Key
    )
    Invoke-WebRequest -Uri "$(Get-IntegrationBaseUrl)/$Engine-$Limiter/$Instance" -Headers @{ "X-Probe-Key" = $Key } -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
}

# Get-RateLimitHeader is one response header as a string (Invoke-WebRequest returns header values as arrays).
function Get-RateLimitHeader {
    param(
        $Response,
        [string]$Name
    )
    return "$($Response.Headers[$Name])"
}

# Assert-RateLimitSharedAcrossInstances spends a limit of 3 across instances a, b, a on one key, then expects b to deny.
# The two instances are separate middleware (separate limiter and SimpleRedis), so the shared count can only live in the engine.
function Assert-RateLimitSharedAcrossInstances {
    param(
        [string]$Engine,
        [string]$Limiter,
        [int]$MaxRetryAfterSeconds
    )
    $key = New-RateLimitKey
    $expectedRemaining = 2
    foreach ($instance in @("a", "b", "a")) {
        $response = Invoke-RateLimitProbe -Engine $Engine -Limiter $Limiter -Instance $instance -Key $key
        $response.StatusCode | Should -Be 200 -Because "instance $instance, remaining $expectedRemaining expected: $($response.Content)"
        (Get-RateLimitHeader $response "X-RateLimit-Limit") | Should -Be "3"
        (Get-RateLimitHeader $response "X-RateLimit-Remaining") | Should -Be "$expectedRemaining"
        (Get-RateLimitHeader $response "Retry-After") | Should -BeNullOrEmpty
        $expectedRemaining--
    }
    $denied = Invoke-RateLimitProbe -Engine $Engine -Limiter $Limiter -Instance "b" -Key $key
    $denied.StatusCode | Should -Be 429 -Because $denied.Content
    $denied.Content.Trim() | Should -Be "ratelimit:denied"
    (Get-RateLimitHeader $denied "X-RateLimit-Limit") | Should -Be "3"
    (Get-RateLimitHeader $denied "X-RateLimit-Remaining") | Should -Be "0"
    $retryAfter = [int](Get-RateLimitHeader $denied "Retry-After")
    $retryAfter | Should -BeGreaterOrEqual 1
    $retryAfter | Should -BeLessOrEqual $MaxRetryAfterSeconds
    [int](Get-RateLimitHeader $denied "X-RateLimit-Reset") | Should -BeGreaterThan 0
}

# Assert-RateLimitKeysIsolated exhausts one key, then expects a fresh key on the same instance to be admitted.
function Assert-RateLimitKeysIsolated {
    param(
        [string]$Engine,
        [string]$Limiter
    )
    $spent = New-RateLimitKey
    1..3 | ForEach-Object {
        $response = Invoke-RateLimitProbe -Engine $Engine -Limiter $Limiter -Instance "a" -Key $spent
        $response.StatusCode | Should -Be 200 -Because $response.Content
    }
    (Invoke-RateLimitProbe -Engine $Engine -Limiter $Limiter -Instance "a" -Key $spent).StatusCode | Should -Be 429
    $fresh = Invoke-RateLimitProbe -Engine $Engine -Limiter $Limiter -Instance "a" -Key (New-RateLimitKey)
    $fresh.StatusCode | Should -Be 200 -Because $fresh.Content
    (Get-RateLimitHeader $fresh "X-RateLimit-Remaining") | Should -Be "2"
}

# Get-EngineKeys lists BackendHost keys matching Pattern (KEYS is fine on the throwaway compose engines).
function Get-EngineKeys {
    param(
        [string]$BackendHost,
        [string]$Pattern
    )
    $output = docker compose -p reclaim-e2e exec -T redis redis-cli -h $BackendHost --raw KEYS $Pattern
    if ($LASTEXITCODE -ne 0) {
        throw "redis-cli -h $BackendHost KEYS $Pattern failed: $output"
    }
    return @($output | Where-Object { "$_".Trim() -ne "" })
}
