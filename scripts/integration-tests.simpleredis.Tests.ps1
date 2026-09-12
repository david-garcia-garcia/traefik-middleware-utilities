. (Join-Path $PSScriptRoot "integration-tests.utils/Import.ps1")

Describe "simpleredis Yaegi e2e" {
    It "Traefik API is reachable" {
        Assert-TraefikApiReachable
    }

    It "GET /<Engine>/get returns the Set body" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb set -Key $key -Ex 60 -Body $key
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $response = Invoke-SimpleRedis -Engine $Engine -Verb get -Key $key
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content | Should -Be $key
    }

    It "GET /<Engine>/mget returns the Set token in slot 0" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb set -Key $key -Ex 60 -Body $key
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $response = Invoke-SimpleRedis -Engine $Engine -Verb mget -Key @($key, "$key-missing")
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $slots = $response.Content -split "`n"
        $slots[0] | Should -Be $key
        $slots[1] | Should -Be ""
    }

    It "POST /<Engine>/del succeeds" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb set -Key $key -Ex 60 -Body "gone"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $del = Invoke-SimpleRedis -Engine $Engine -Verb del -Key $key
        $del.StatusCode | Should -Be 200 -Because $del.Content
        $get = Invoke-SimpleRedis -Engine $Engine -Verb get -Key $key
        $get.StatusCode | Should -Be 502
        $get.Content.Trim() | Should -Be "redis:miss"
    }

    It "POST /<Engine>/incr returns 1" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $response = Invoke-SimpleRedis -Engine $Engine -Verb incr -Key (New-SimpleRedisKey)
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content.Trim() | Should -Be "1"
    }

    It "POST /<Engine>/incrby returns 5" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $response = Invoke-SimpleRedis -Engine $Engine -Verb incrby -Key (New-SimpleRedisKey) -Delta 5
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content.Trim() | Should -Be "5"
    }

    It "POST /<Engine>/expire succeeds" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb set -Key $key -Ex 60 -Body "1"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $expire = Invoke-SimpleRedis -Engine $Engine -Verb expire -Key $key -Ex 30
        $expire.StatusCode | Should -Be 200 -Because $expire.Content
    }

    It "POST /<Engine>/expireat succeeds" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb set -Key $key -Ex 60 -Body "1"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $at = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        $expire = Invoke-SimpleRedis -Engine $Engine -Verb expireat -Key $key -At $at
        $expire.StatusCode | Should -Be 200 -Because $expire.Content
    }

    It "POST /<Engine>/eval returns 3" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $expireUnix = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        $response = Invoke-SimpleRedis -Engine $Engine -Verb eval -Key $key -Arg @("3", $expireUnix) -Digest $script:KongEvalDigest -Body $script:KongIncrbyExpireatScript
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Content.Trim() | Should -Be "3"
    }

    It "GET /<Engine>/get of a missing key is redis:miss" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $response = Invoke-SimpleRedis -Engine $Engine -Verb get -Key (New-SimpleRedisKey)
        $response.StatusCode | Should -Be 502
        $response.Content.Trim() | Should -Be "redis:miss"
    }

    It "POST /<Engine>/msetex lands a positive TTL" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb msetex -Key $key -Ex 60 -Body "ok"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $ttl = Invoke-SimpleRedis -Engine $Engine -Verb eval -Key $key -Digest $script:TtlEvalDigest -Body $script:TtlScript
        $ttl.StatusCode | Should -Be 200 -Because $ttl.Content
        [int]$ttl.Content.Trim() | Should -BeGreaterThan 0
    }

    It "POST /<Engine>/msetexat returns the written value" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $key = New-SimpleRedisKey
        $at = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        $set = Invoke-SimpleRedis -Engine $Engine -Verb msetexat -Key $key -At $at -Body "ok"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $get = Invoke-SimpleRedis -Engine $Engine -Verb get -Key $key
        $get.StatusCode | Should -Be 200 -Because $get.Content
        $get.Content | Should -Be "ok"
    }

    It "drop=1 on /<Engine> double-applies Incr and Eval" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $incrKey = New-SimpleRedisKey
        $evalKey = New-SimpleRedisKey
        $expireUnix = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
        Invoke-SimpleRedis -Engine $Engine -Verb get -Key "$incrKey-warm" -Drop | Out-Null
        $incr = Invoke-SimpleRedis -Engine $Engine -Verb incr -Key $incrKey -Drop
        $incr.StatusCode | Should -Be 200 -Because $incr.Content
        $incr.Content.Trim() | Should -Be "2"
        $incrStored = Invoke-SimpleRedis -Engine $Engine -Verb get -Key $incrKey
        $incrStored.StatusCode | Should -Be 200 -Because $incrStored.Content
        $incrStored.Content.Trim() | Should -Be "2"
        Invoke-SimpleRedis -Engine $Engine -Verb get -Key "$evalKey-warm" -Drop | Out-Null
        $eval = Invoke-SimpleRedis -Engine $Engine -Verb eval -Key $evalKey -Arg @("3", $expireUnix) -Digest $script:KongEvalDigest -Body $script:KongIncrbyExpireatScript -Drop
        $eval.StatusCode | Should -Be 200 -Because $eval.Content
        $eval.Content.Trim() | Should -Be "6"
        $evalStored = Invoke-SimpleRedis -Engine $Engine -Verb get -Key $evalKey
        $evalStored.StatusCode | Should -Be 200 -Because $evalStored.Content
        $evalStored.Content.Trim() | Should -Be "6"
    }

    It "POST /<Engine>/eval proves EVALSHA miss then hit without stopping whoami-a or whoami-b" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        Assert-EvalShaMissThenHit -Engine $Engine -BackendHost $BackendHost
    }

    It "GET /<Engine>-wrong-password returns 502 redis:noauth" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/$Engine-wrong-password" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content.Trim() | Should -Be "redis:noauth"
    }

    It "GET /<Engine>-database-99 returns 502 ERR DB index is out of range" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/$Engine-database-99" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content | Should -Match "ERR DB index is out of range"
    }

    It "two GET /<Engine> health return distinct own-values" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        Assert-TwoDistinctHealthValues -Url "$script:BaseUrl/$Engine"
    }

    It "POST /<Engine>/set then GET recovers after CLIENT KILL of the Traefik client" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        $warmup = Invoke-WebRequest -Uri "$script:BaseUrl/$Engine" -UseBasicParsing -TimeoutSec 10
        $warmup.StatusCode | Should -Be 200
        Stop-TraefikEngineClientForTest -Engine $Engine
        $key = New-SimpleRedisKey
        $set = Invoke-SimpleRedis -Engine $Engine -Verb set -Key $key -Ex 60 -Body "ok"
        $set.StatusCode | Should -Be 200 -Because $set.Content
        $get = Invoke-SimpleRedis -Engine $Engine -Verb get -Key $key
        $get.StatusCode | Should -Be 200 -Because $get.Content
        $get.Content | Should -Be "ok"
    }

    It "POST /<Engine>/eval concurrent holds stay within default poolSize" -TestCases $script:SimpleRedisEngines {
        param($Engine, $BackendHost)
        Assert-SimpleRedisLiveCap -Path "/$Engine/eval" -BackendHost $BackendHost
    }
}
