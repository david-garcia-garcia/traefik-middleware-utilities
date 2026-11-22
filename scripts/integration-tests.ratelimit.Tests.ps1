BeforeAll {
    . (Join-Path $PSScriptRoot "integration-tests.utils/Import.ps1")
    $script:Engine = $env:INTEGRATION_ENGINE
    if ($script:Engine -notin @("redis", "dragonfly", "valkey")) {
        throw "INTEGRATION_ENGINE must be redis, dragonfly or valkey"
    }
}

# ratelimitprobe under Traefik's Yaegi: tokenbucket is rate 0.1/s burst 3, windowcounter is limit 3 per 60s, both with key
# prefix shared by instances a and b (docker-compose.yml whoami-ratelimit-<engine>). Limiter math stays in Go E2E.
Describe "rate limiters through Traefik ($env:INTEGRATION_ENGINE)" {
    It "Traefik API is reachable" {
        Assert-TraefikApiReachable
    }

    It "tokenbucket on /$env:INTEGRATION_ENGINE-tokenbucket shares one budget across two middleware instances" {
        Assert-RateLimitSharedAcrossInstances -Engine $script:Engine -Limiter "tokenbucket" -MaxRetryAfterSeconds 10
    }

    It "tokenbucket on /$env:INTEGRATION_ENGINE-tokenbucket keeps keys apart" {
        Assert-RateLimitKeysIsolated -Engine $script:Engine -Limiter "tokenbucket"
    }

    It "tokenbucket stores its bucket in $env:INTEGRATION_ENGINE under the key prefix" {
        $key = New-RateLimitKey
        $response = Invoke-RateLimitProbe -Engine $script:Engine -Limiter "tokenbucket" -Instance "a" -Key $key
        $response.StatusCode | Should -Be 200 -Because $response.Content
        (Get-EngineKeys -BackendHost $script:Engine -Pattern "tb:$key") | Should -Be "tb:$key"
    }

    It "windowcounter on /$env:INTEGRATION_ENGINE-windowcounter shares one window across two middleware instances" {
        # Hits late in one 60s window still weigh on the next one, so the wait can reach two windows.
        Assert-RateLimitSharedAcrossInstances -Engine $script:Engine -Limiter "windowcounter" -MaxRetryAfterSeconds 120
    }

    It "windowcounter on /$env:INTEGRATION_ENGINE-windowcounter keeps keys apart" {
        Assert-RateLimitKeysIsolated -Engine $script:Engine -Limiter "windowcounter"
    }

    It "windowcounter stores its window counter in $env:INTEGRATION_ENGINE under the key prefix" {
        $key = New-RateLimitKey
        $response = Invoke-RateLimitProbe -Engine $script:Engine -Limiter "windowcounter" -Instance "a" -Key $key
        $response.StatusCode | Should -Be 200 -Because $response.Content
        @(Get-EngineKeys -BackendHost $script:Engine -Pattern "wc:$key*").Count | Should -BeGreaterOrEqual 1
    }
}
