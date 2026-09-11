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

    function Get-BackendIdleClients {
        param(
            [string]$BackendHost
        )
        $raw = docker compose exec -T redis redis-cli -h $BackendHost CLIENT LIST 2>&1 | Out-String
        if ($LASTEXITCODE -ne 0) {
            throw "CLIENT LIST on $BackendHost failed: $raw"
        }
        $lines = @($raw -split "[\r\n]+" | Where-Object { $_ -match 'id=' })
        return [Math]::Max(0, $lines.Count - 1)
    }
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

    It "GET /redis overlap leaves at most eight idle sockets" {
        $baseUrl = $script:BaseUrl
        $results = 1..16 | ForEach-Object -Parallel {
            Invoke-WebRequest -Uri "$using:baseUrl/redis" -UseBasicParsing -TimeoutSec 30
        } -ThrottleLimit 16
        foreach ($response in $results) {
            $response.StatusCode | Should -Be 200
        }
        $remaining = 99
        $elapsed = 0
        do {
            $remaining = Get-BackendIdleClients -BackendHost redis
            if ($remaining -le 8) {
                break
            }
            Start-Sleep -Milliseconds 200
            $elapsed += 200
        } while ($elapsed -lt 2000)
        $remaining | Should -BeLessOrEqual 8
    }

    It "GET /dragonfly overlap leaves at most eight idle sockets" {
        $baseUrl = $script:BaseUrl
        $results = 1..16 | ForEach-Object -Parallel {
            Invoke-WebRequest -Uri "$using:baseUrl/dragonfly" -UseBasicParsing -TimeoutSec 30
        } -ThrottleLimit 16
        foreach ($response in $results) {
            $response.StatusCode | Should -Be 200
        }
        $remaining = 99
        $elapsed = 0
        do {
            $remaining = Get-BackendIdleClients -BackendHost dragonfly
            if ($remaining -le 8) {
                break
            }
            Start-Sleep -Milliseconds 200
            $elapsed += 200
        } while ($elapsed -lt 2000)
        $remaining | Should -BeLessOrEqual 8
    }
}
