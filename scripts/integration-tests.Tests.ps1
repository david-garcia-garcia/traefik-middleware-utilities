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

    function Get-SimpleRedisHeader {
        param(
            $Response,
            [string]$Name
        )
        return [string]@($Response.Headers[$Name])[0]
    }

    function Assert-SimpleRedisVerbs {
        param($Response)
        $Response.StatusCode | Should -Be 200
        $value = Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-Value"
        $value | Should -Match '^srp:\d+$'
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-MGet") | Should -Be $value
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-Del") | Should -Be "ok"
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-Incr") | Should -Be "1"
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-IncrBy") | Should -Be "5"
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-Expire") | Should -Be "ok"
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-ExpireAt") | Should -Be "ok"
        (Get-SimpleRedisHeader -Response $Response -Name "X-SimpleRedis-Eval") | Should -Be "3"
    }

    function Invoke-TwoOverlappingGets {
        param([string]$Url)
        $fetch = {
            param($Uri)
            Invoke-WebRequest -Uri $Uri -UseBasicParsing -TimeoutSec 10
        }
        $jobA = Start-Job -ScriptBlock $fetch -ArgumentList $Url
        $jobB = Start-Job -ScriptBlock $fetch -ArgumentList $Url
        $a = Receive-Job $jobA -Wait
        $b = Receive-Job $jobB -Wait
        Remove-Job $jobA, $jobB -Force
        return @($a, $b)
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
        Assert-SimpleRedisVerbs $response
    }

    It "GET /dragonfly succeeds and echoes every SimpleRedis verb" {
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/dragonfly" -UseBasicParsing -TimeoutSec 10
        Assert-SimpleRedisVerbs $response
    }

    It "two overlapping GET /redis return distinct own-values" {
        $responses = Invoke-TwoOverlappingGets -Url "$script:BaseUrl/redis"
        $responses.Count | Should -Be 2
        $a = Get-SimpleRedisHeader -Response $responses[0] -Name "X-SimpleRedis-Value"
        $b = Get-SimpleRedisHeader -Response $responses[1] -Name "X-SimpleRedis-Value"
        $a | Should -Match '^srp:\d+$'
        $b | Should -Match '^srp:\d+$'
        $a | Should -Not -Be $b
        (Get-SimpleRedisHeader -Response $responses[0] -Name "X-SimpleRedis-MGet") | Should -Be $a
        (Get-SimpleRedisHeader -Response $responses[1] -Name "X-SimpleRedis-MGet") | Should -Be $b
    }

    It "two overlapping GET /dragonfly return distinct own-values" {
        $responses = Invoke-TwoOverlappingGets -Url "$script:BaseUrl/dragonfly"
        $responses.Count | Should -Be 2
        $a = Get-SimpleRedisHeader -Response $responses[0] -Name "X-SimpleRedis-Value"
        $b = Get-SimpleRedisHeader -Response $responses[1] -Name "X-SimpleRedis-Value"
        $a | Should -Match '^srp:\d+$'
        $b | Should -Match '^srp:\d+$'
        $a | Should -Not -Be $b
        (Get-SimpleRedisHeader -Response $responses[0] -Name "X-SimpleRedis-MGet") | Should -Be $a
        (Get-SimpleRedisHeader -Response $responses[1] -Name "X-SimpleRedis-MGet") | Should -Be $b
    }
}
