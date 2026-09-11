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

    function script:Wait-BackendPing {
        param([string]$BackendHost)
        docker compose -p reclaim-e2e start redis $BackendHost | Out-Null
        $deadline = [DateTime]::UtcNow.AddSeconds(30)
        do {
            $pong = docker compose -p reclaim-e2e exec -T redis redis-cli -h $BackendHost ping 2>$null
            if ("$pong".Trim() -eq "PONG") {
                return
            }
            Start-Sleep -Milliseconds 400
        } while ([DateTime]::UtcNow -lt $deadline)
        throw "$BackendHost did not answer PING"
    }

    function script:Start-NetnsReader {
        param([string]$BackendHost)
        if ($BackendHost -eq "redis") {
            return $null
        }
        $id = (docker compose -p reclaim-e2e ps -q $BackendHost).Trim()
        $id | Should -Not -BeNullOrEmpty -Because "$BackendHost container id"
        $name = "reclaim-e2e-netcount-$BackendHost"
        docker rm -f $name 2>$null | Out-Null
        docker run -d --name $name --network "container:$id" redis:7-alpine sleep 120 | Out-Null
        return $name
    }

    function script:Read-BackendTcp {
        param(
            [string]$BackendHost,
            [string]$SidecarName
        )
        if ($BackendHost -eq "redis") {
            $tcp4 = docker compose -p reclaim-e2e exec -T redis cat /proc/net/tcp 2>$null
            $tcp6 = docker compose -p reclaim-e2e exec -T redis cat /proc/net/tcp6 2>$null
            return "$tcp4`n$tcp6"
        }
        $tcp4 = docker exec $SidecarName cat /proc/net/tcp 2>$null
        $tcp6 = docker exec $SidecarName cat /proc/net/tcp6 2>$null
        return "$tcp4`n$tcp6"
    }

    function script:Count-Established6379 {
        param($TcpDump)
        return [regex]::Matches("$TcpDump", ':18[Ee][Bb]\s+\S+\s+01\s').Count
    }

    function script:Assert-SimpleRedisLiveCap {
        param(
            [string]$Path,
            [string]$BackendHost
        )
        Wait-BackendPing -BackendHost $BackendHost
        $sidecar = Start-NetnsReader -BackendHost $BackendHost
        $holdUrl = "$script:BaseUrl$Path`?hold=500000"
        $http = [System.Net.Http.HttpClient]::new()
        $http.Timeout = [TimeSpan]::FromSeconds(15)
        try {
            $holds = 1..8 | ForEach-Object { $http.GetAsync($holdUrl) }
            $deadline = [DateTime]::UtcNow.AddMilliseconds(400)
            $live = 0
            $tcpDump = ""
            do {
                Start-Sleep -Milliseconds 40
                $tcpDump = Read-BackendTcp -BackendHost $BackendHost -SidecarName $sidecar
                $live = Count-Established6379 -TcpDump $tcpDump
            } while ($live -lt 8 -and [DateTime]::UtcNow -lt $deadline)
            $live | Should -BeGreaterOrEqual 8 -Because "tcp dump was: $tcpDump"
            $live | Should -BeLessOrEqual 10 -Because "tcp dump was: $tcpDump"
            $ninth = $http.GetAsync($holdUrl).GetAwaiter().GetResult()
            [int]$ninth.StatusCode | Should -Be 502
            $ninthBody = $ninth.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            $ninthBody | Should -Match "redis:unreachable"
            foreach ($hold in $holds) {
                $completed = $hold.GetAwaiter().GetResult()
                [int]$completed.StatusCode | Should -BeIn @(200, 502)
            }
        }
        finally {
            $http.Dispose()
            if ($sidecar) {
                docker rm -f $sidecar 2>$null | Out-Null
            }
        }
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
        docker compose -p reclaim-e2e stop whoami-a whoami-b
        $LASTEXITCODE | Should -Be 0
        Wait-TraefikPluginLog -Pattern "reclaim_orphan" | Should -BeTrue
        Wait-TraefikPluginLog -Pattern "reclaimprobe_sleep" | Should -BeTrue
        docker compose -p reclaim-e2e start whoami-a whoami-b
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
        docker compose -p reclaim-e2e stop whoami-a whoami-b
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

    It "GET /redis concurrent holds stay within eight live clients" {
        Assert-SimpleRedisLiveCap -Path "/redis" -BackendHost "redis"
    }

    It "GET /dragonfly concurrent holds stay within eight live clients" {
        Assert-SimpleRedisLiveCap -Path "/dragonfly" -BackendHost "dragonfly"
    }
}
