BeforeAll {
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

    $script:BaseUrl = "http://localhost:8000"
    $script:TraefikApiUrl = "http://localhost:8080"
}

Describe "reclaim Yaegi e2e" {
    It "Traefik API is reachable" {
        $response = Invoke-WebRequest -Uri "$script:TraefikApiUrl/api/rawdata" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
    }

    It "both routes succeed and share one reclaim incarnation" {
        $a = Invoke-WebRequest -Uri "$script:BaseUrl/a" -UseBasicParsing -TimeoutSec 10
        $b = Invoke-WebRequest -Uri "$script:BaseUrl/b" -UseBasicParsing -TimeoutSec 10
        $a.StatusCode | Should -Be 200
        $b.StatusCode | Should -Be 200
        $idA = $a.Headers["X-Reclaim-ID"]
        $idB = $b.Headers["X-Reclaim-ID"]
        $idA | Should -Not -BeNullOrEmpty
        $idA | Should -Be $idB
    }

    It "Traefik logs reclaim_put and reclaim_bind" {
        Wait-TraefikPluginLog -Pattern "reclaim_put" | Should -BeTrue
        Wait-TraefikPluginLog -Pattern "reclaim_bind" | Should -BeTrue
    }

    It "reload sleeps then wakes the shared incarnation" {
        docker compose stop whoami-a whoami-b
        $LASTEXITCODE | Should -Be 0
        Wait-TraefikPluginLog -Pattern "reclaim_orphan" | Should -BeTrue
        Wait-TraefikPluginLog -Pattern "reclaimprobe_sleep" | Should -BeTrue
        docker compose start whoami-a whoami-b
        $LASTEXITCODE | Should -Be 0
        Wait-TraefikPluginLog -Pattern "reclaim_reclaim" | Should -BeTrue
        Wait-TraefikPluginLog -Pattern "reclaimprobe_wake" | Should -BeTrue
        $ready = $false
        $elapsed = 0
        do {
            try {
                $a = Invoke-WebRequest -Uri "$script:BaseUrl/a" -UseBasicParsing -TimeoutSec 10
                $b = Invoke-WebRequest -Uri "$script:BaseUrl/b" -UseBasicParsing -TimeoutSec 10
                if ($a.StatusCode -eq 200 -and $b.StatusCode -eq 200) {
                    $ready = $true
                    break
                }
            }
            catch {
                # whoami not ready yet
            }
            Start-Sleep 2
            $elapsed += 2
        } while ($elapsed -lt 60)
        $ready | Should -BeTrue
    }

    It "teardown disposes after grace and runs the close hook" {
        docker compose stop whoami-a whoami-b
        $LASTEXITCODE | Should -Be 0
        Start-Sleep 12
        Wait-TraefikPluginLog -Pattern "reclaim_dispose" -TimeoutSeconds 30 | Should -BeTrue
        Wait-TraefikPluginLog -Pattern "reclaimprobe_close" -TimeoutSeconds 30 | Should -BeTrue
    }
}

Describe "simpleredis Yaegi e2e" {
    It "Traefik API is reachable" {
        $response = Invoke-WebRequest -Uri "$script:TraefikApiUrl/api/rawdata" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
    }

    It "GET /redis succeeds and echoes every SimpleRedis verb" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/redis" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
        $response.Headers["X-SimpleRedis-Value"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-MGet"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-Del"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-Incr"] | Should -Be "1"
        $response.Headers["X-SimpleRedis-IncrBy"] | Should -Be "5"
        $response.Headers["X-SimpleRedis-Expire"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-ExpireAt"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
    }

    It "GET /dragonfly succeeds and echoes every SimpleRedis verb" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/dragonfly" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
        $response.Headers["X-SimpleRedis-Value"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-MGet"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-Del"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-Incr"] | Should -Be "1"
        $response.Headers["X-SimpleRedis-IncrBy"] | Should -Be "5"
        $response.Headers["X-SimpleRedis-Expire"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-ExpireAt"] | Should -Be "ok"
        $response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
    }

    It "GET /redis-wrong-password returns 502 redis:noauth" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/redis-wrong-password" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content.Trim() | Should -Be "redis:noauth"
    }

    It "GET /dragonfly-wrong-password returns 502 redis:noauth" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/dragonfly-wrong-password" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content.Trim() | Should -Be "redis:noauth"
    }

    It "GET /redis-database-99 returns 502 ERR DB index is out of range" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/redis-database-99" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content | Should -Match "ERR DB index is out of range"
    }

    It "GET /dragonfly-database-99 returns 502 ERR DB index is out of range" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/dragonfly-database-99" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
        $response.StatusCode | Should -Be 502
        $response.Content | Should -Match "ERR DB index is out of range"
    }
}
