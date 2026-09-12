. (Join-Path $PSScriptRoot "integration-tests.utils/Import.ps1")

$script:Engine = $env:INTEGRATION_ENGINE
if ($script:Engine -notin @("redis", "dragonfly")) {
    throw "INTEGRATION_ENGINE must be redis or dragonfly"
}
$script:BackendHost = $script:Engine

Describe "simpleredis Yaegi e2e ($($script:Engine))" {
    It "Traefik API is reachable" {
        Assert-TraefikApiReachable
    }

    It "GET /$($script:Engine)/get returns the Set body" {
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb set -Key $key -Ex 60 -Body $key
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $response = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key $key
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content | Should -Be $key
    }

    It "GET /$($script:Engine)/mget returns the Set token in slot 0" {
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb set -Key $key -Ex 60 -Body $key
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $response = Invoke-SimpleRedis -Engine $script:Engine -Verb mget -Key @($key, "$key-missing")
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $slots = $response.Content -split "`n"
        $slots[0] | Should -Be $key
        $slots[1] | Should -Be ""
    }

    It "POST /$($script:Engine)/del succeeds" {
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb set -Key $key -Ex 60 -Body "gone"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $del = Invoke-SimpleRedis -Engine $script:Engine -Verb del -Key $key
        $del.StatusCode | Should -Be 200 -Because $del.Content
        $get = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key $key
        $get.StatusCode | Should -Be 502
        $get.Content.Trim() | Should -Be "redis:miss"
    }

    It "POST /$($script:Engine)/incr returns 1" {
        $response = Invoke-SimpleRedis -Engine $script:Engine -Verb incr -Key (New-SimpleRedisKey)
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content.Trim() | Should -Be "1"
    }

    It "POST /$($script:Engine)/incrby returns 5" {
        $response = Invoke-SimpleRedis -Engine $script:Engine -Verb incrby -Key (New-SimpleRedisKey) -Delta 5
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content.Trim() | Should -Be "5"
    }

    It "POST /$($script:Engine)/expire succeeds" {
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb set -Key $key -Ex 60 -Body "1"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $expire = Invoke-SimpleRedis -Engine $script:Engine -Verb expire -Key $key -Ex 30
        $expire.StatusCode | Should -Be 200 -Because $expire.Content
    }

    It "POST /$($script:Engine)/expireat succeeds" {
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb set -Key $key -Ex 60 -Body "1"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $at = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        $expire = Invoke-SimpleRedis -Engine $script:Engine -Verb expireat -Key $key -At $at
        $expire.StatusCode | Should -Be 200 -Because $expire.Content
    }

    It "POST /$($script:Engine)/eval returns 3" {
        $key = New-SimpleRedisKey
        $expireUnix = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        $response = Invoke-SimpleRedis -Engine $script:Engine -Verb eval -Key $key -Arg @("3", $expireUnix) -Digest $KongEvalDigest -Body $KongIncrbyExpireatScript
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content.Trim() | Should -Be "3"
    }

    It "GET /$($script:Engine)/get of a missing key is redis:miss" {
        $response = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key (New-SimpleRedisKey)
        $response.StatusCode | Should -Be 502
        $response.Content.Trim() | Should -Be "redis:miss"
    }

    It "POST /$($script:Engine)/msetex lands a positive TTL" {
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb msetex -Key $key -Ex 60 -Body "ok"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $ttl = Invoke-SimpleRedis -Engine $script:Engine -Verb eval -Key $key -Digest $TtlEvalDigest -Body $TtlScript
        $ttl.StatusCode | Should -Be 200 -Because $ttl.Content
        [int]$ttl.Content.Trim() | Should -BeGreaterThan 0
    }

    It "POST /$($script:Engine)/msetexat returns the written value" {
        $key = New-SimpleRedisKey
        $at = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb msetexat -Key $key -At $at -Body "ok"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $get = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key $key
        $get.StatusCode | Should -Be 200 -Because $get.Content
        $get.Content | Should -Be "ok"
    }

    It "drop=1 on /$($script:Engine) double-applies Incr and Eval" {
        $incrKey = New-SimpleRedisKey
        $evalKey = New-SimpleRedisKey
        $expireUnix = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key "$incrKey-warm" -Drop | Out-Null
        $incr = Invoke-SimpleRedis -Engine $script:Engine -Verb incr -Key $incrKey -Drop
        $incr.StatusCode | Should -Be 200 -Because $incr.Content
        $incr.Content.Trim() | Should -Be "2"
        $incrStored = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key $incrKey
        $incrStored.StatusCode | Should -Be 200 -Because $incrStored.Content
        $incrStored.Content.Trim() | Should -Be "2"
        Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key "$evalKey-warm" -Drop | Out-Null
        $eval = Invoke-SimpleRedis -Engine $script:Engine -Verb eval -Key $evalKey -Arg @("3", $expireUnix) -Digest $KongEvalDigest -Body $KongIncrbyExpireatScript -Drop
        $eval.StatusCode | Should -Be 200 -Because $eval.Content
        $eval.Content.Trim() | Should -Be "6"
        $evalStored = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key $evalKey
        $evalStored.StatusCode | Should -Be 200 -Because $evalStored.Content
        $evalStored.Content.Trim() | Should -Be "6"
    }

    It "POST /$($script:Engine)/eval proves EVALSHA miss then hit without stopping whoami-a or whoami-b" {
        Assert-EvalShaMissThenHit -Engine $script:Engine -BackendHost $script:BackendHost
    }

    It "GET /$($script:Engine)-wrong-password returns 502 redis:noauth" {
        $response = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/$($script:Engine)-wrong-password" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content.Trim() | Should -Be "redis:noauth"
    }

    It "GET /$($script:Engine)-database-99 returns 502 ERR DB index is out of range" {
        $response = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/$($script:Engine)-database-99" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content | Should -Match "ERR DB index is out of range"
    }

    It "two GET /$($script:Engine) health return distinct own-values" {
        Assert-TwoDistinctHealthValues -Url "$env:INTEGRATION_BASE_URL/$($script:Engine)"
    }

    It "POST /$($script:Engine)/set then GET recovers after CLIENT KILL of the Traefik client" {
        $warmup = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/$($script:Engine)" -UseBasicParsing -TimeoutSec 10
        $warmup.StatusCode | Should -Be 200
        Stop-TraefikEngineClientForTest -Engine $script:Engine
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $script:Engine -Verb set -Key $key -Ex 60 -Body "ok"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $get = Invoke-SimpleRedis -Engine $script:Engine -Verb get -Key $key
        $get.StatusCode | Should -Be 200 -Because $get.Content
        $get.Content | Should -Be "ok"
    }

    It "POST /$($script:Engine)/eval concurrent holds stay within default poolSize" {
        Assert-SimpleRedisLiveCap -Path "/$($script:Engine)/eval" -BackendHost $script:BackendHost
    }
}
