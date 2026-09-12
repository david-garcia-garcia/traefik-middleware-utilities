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
    # SHA-1 of e2e/simpleredisprobe kongIncrbyExpireatScript (crypto/sha1 lowercase hex).
    $script:KongEvalDigest = "ff5f5f110f45a4519e6747630d51c5a5687e9608"

    function Get-SimpleRedisHeader {
        param(
            $Response,
            [string]$Name
        )
        return [string]@($Response.Headers[$Name])[0]
    }

    function Invoke-RedisCli {
        param(
            [string]$BackendHost,
            [string[]]$CliArgs
        )
        $output = docker compose -p reclaim-e2e exec -T redis redis-cli -h $BackendHost --raw @CliArgs
        if ($LASTEXITCODE -ne 0) {
            throw "redis-cli -h $BackendHost $($CliArgs -join ' ') failed: $output"
        }
        return "$output".Trim()
    }

    function Clear-ScriptCache {
        param([string]$BackendHost)
        Invoke-RedisCli -BackendHost $BackendHost -CliArgs @("SCRIPT", "FLUSH") | Out-Null
    }

    function Get-ScriptExists {
        param(
            [string]$BackendHost,
            [string]$Digest
        )
        Invoke-RedisCli -BackendHost $BackendHost -CliArgs @("SCRIPT", "EXISTS", $Digest)
    }

    function Invoke-SimpleRedisCase {
        param(
            [string]$Engine,
            [string]$Case
        )
        Invoke-WebRequest -Uri "$script:BaseUrl/$Engine/$Case" -UseBasicParsing -TimeoutSec 10 -SkipHttpErrorCheck
    }

    function Assert-SimpleRedisHeaderCase {
        param(
            [string]$Engine,
            [string]$Case,
            [string]$Header,
            [string]$Expected
        )
        $response = Invoke-SimpleRedisCase -Engine $Engine -Case $Case
        $response.StatusCode | Should -Be 200 -Because $response.Content
        (Get-SimpleRedisHeader -Response $response -Name $Header) | Should -Be $Expected
    }

    function Assert-EvalShaMissThenHit {
        param(
            [string]$Route,
            [string]$BackendHost
        )
        Clear-ScriptCache -BackendHost $BackendHost
        (Get-ScriptExists -BackendHost $BackendHost -Digest $script:KongEvalDigest) | Should -Be "0"
        $response = Invoke-WebRequest -Uri "$script:BaseUrl$Route" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
        $response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
        $response.Headers["X-SimpleRedis-EvalDigest"] | Should -Be $script:KongEvalDigest
        (Get-ScriptExists -BackendHost $BackendHost -Digest $script:KongEvalDigest) | Should -Be "1"
        $again = Invoke-WebRequest -Uri "$script:BaseUrl$Route" -UseBasicParsing -TimeoutSec 10
        $again.StatusCode | Should -Be 200
        $again.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
    }

    function Assert-TwoDistinctOwnValues {
        param([string]$Url)
        $first = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 15 -SkipHttpErrorCheck
        $second = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 15 -SkipHttpErrorCheck
        $first.StatusCode | Should -Be 200 -Because $first.Content
        $second.StatusCode | Should -Be 200 -Because $second.Content
        $a = Get-SimpleRedisHeader -Response $first -Name "X-SimpleRedis-Value"
        $b = Get-SimpleRedisHeader -Response $second -Name "X-SimpleRedis-Value"
        $a | Should -Match '^srp:\d+$'
        $b | Should -Match '^srp:\d+$'
        $a | Should -Not -Be $b
    }

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
            # Default liveCap is 8; fill poolSize then the waiter is redis:unreachable.
            $holds = 1..8 | ForEach-Object { $http.GetAsync($holdUrl) }
            $deadline = [DateTime]::UtcNow.AddMilliseconds(400)
            $established = 0
            $tcpDump = ""
            do {
                Start-Sleep -Milliseconds 40
                $tcpDump = Read-BackendTcp -BackendHost $BackendHost -SidecarName $sidecar
                $established = Count-Established6379 -TcpDump $tcpDump
            } while ($established -lt 8 -and [DateTime]::UtcNow -lt $deadline)
            $established | Should -BeGreaterOrEqual 8 -Because "tcp dump was: $tcpDump"
            $established | Should -BeLessOrEqual 10 -Because "tcp dump was: $tcpDump"
            $waiter = $http.GetAsync($holdUrl).GetAwaiter().GetResult()
            [int]$waiter.StatusCode | Should -Be 502
            $waiterBody = $waiter.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            $waiterBody | Should -Match "redis:unreachable"
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

    # Stop-TraefikEngineClientForTest CLIENT KILLs Traefik ADDR on Redis or Dragonfly (not TYPE/SKIPME).
    function Stop-TraefikEngineClientForTest {
        param(
            [Parameter(Mandatory)]
            [ValidateSet("redis", "dragonfly")]
            [string]$Engine
        )
        # Traefik compose IP so CLIENT LIST can match addr=<ip>:<port>.
        $traefikIp = (docker inspect -f "{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}" reclaim-e2e-traefik).Trim()
        if (-not $traefikIp) {
            throw "traefik container IP is empty"
        }
        $cli = @("compose", "-p", "reclaim-e2e", "exec", "-T", "redis", "redis-cli")
        if ($Engine -eq "dragonfly") {
            $cli += @("-h", "dragonfly")
        }
        # CLIENT LIST on the engine (Dragonfly via redis-cli -h dragonfly).
        $list = docker @cli CLIENT LIST
        if ($LASTEXITCODE -ne 0) {
            throw "CLIENT LIST on $Engine failed"
        }
        # CLIENT KILL ADDR for each Traefik client; Dragonfly has no TYPE/SKIPME.
        $killed = 0
        foreach ($line in ($list -split "`r?`n")) {
            if ($line -match "addr=$([regex]::Escape($traefikIp)):\d+") {
                $addr = $Matches[0].Substring("addr=".Length)
                docker @cli CLIENT KILL ADDR $addr | Out-Null
                if ($LASTEXITCODE -ne 0) {
                    throw "CLIENT KILL ADDR $addr on $Engine failed"
                }
                $killed++
            }
        }
        if ($killed -eq 0) {
            throw "CLIENT KILL killed 0 Traefik clients on $Engine (ip $traefikIp). CLIENT LIST: $list"
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

    It "GET /redis/get returns unique Set token" {
        $response = Invoke-SimpleRedisCase -Engine redis -Case get
        $response.StatusCode | Should -Be 200 -Because $response.Content
        (Get-SimpleRedisHeader -Response $response -Name "X-SimpleRedis-Value") | Should -Match '^srp:\d+$'
    }

    It "GET /dragonfly/get returns unique Set token" {
        $response = Invoke-SimpleRedisCase -Engine dragonfly -Case get
        $response.StatusCode | Should -Be 200 -Because $response.Content
        (Get-SimpleRedisHeader -Response $response -Name "X-SimpleRedis-Value") | Should -Match '^srp:\d+$'
    }

    It "GET /redis/mget returns the Set token in slot 0" {
        $response = Invoke-SimpleRedisCase -Engine redis -Case mget
        $response.StatusCode | Should -Be 200 -Because $response.Content
        (Get-SimpleRedisHeader -Response $response -Name "X-SimpleRedis-MGet") | Should -Match '^srp:\d+$'
    }

    It "GET /dragonfly/mget returns the Set token in slot 0" {
        $response = Invoke-SimpleRedisCase -Engine dragonfly -Case mget
        $response.StatusCode | Should -Be 200 -Because $response.Content
        (Get-SimpleRedisHeader -Response $response -Name "X-SimpleRedis-MGet") | Should -Match '^srp:\d+$'
    }

    It "GET /redis/del succeeds" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case del -Header "X-SimpleRedis-Del" -Expected "ok"
    }

    It "GET /dragonfly/del succeeds" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case del -Header "X-SimpleRedis-Del" -Expected "ok"
    }

    It "GET /redis/incr returns 1" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case incr -Header "X-SimpleRedis-Incr" -Expected "1"
    }

    It "GET /dragonfly/incr returns 1" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case incr -Header "X-SimpleRedis-Incr" -Expected "1"
    }

    It "GET /redis/incrby returns 5" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case incrby -Header "X-SimpleRedis-IncrBy" -Expected "5"
    }

    It "GET /dragonfly/incrby returns 5" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case incrby -Header "X-SimpleRedis-IncrBy" -Expected "5"
    }

    It "GET /redis/expire succeeds" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case expire -Header "X-SimpleRedis-Expire" -Expected "ok"
    }

    It "GET /dragonfly/expire succeeds" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case expire -Header "X-SimpleRedis-Expire" -Expected "ok"
    }

    It "GET /redis/expireat succeeds" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case expireat -Header "X-SimpleRedis-ExpireAt" -Expected "ok"
    }

    It "GET /dragonfly/expireat succeeds" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case expireat -Header "X-SimpleRedis-ExpireAt" -Expected "ok"
    }

    It "GET /redis/eval returns 3 and the Kong digest" {
        $response = Invoke-SimpleRedisCase -Engine redis -Case eval
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
        $response.Headers["X-SimpleRedis-EvalDigest"] | Should -Be $script:KongEvalDigest
    }

    It "GET /dragonfly/eval returns 3 and the Kong digest" {
        $response = Invoke-SimpleRedisCase -Engine dragonfly -Case eval
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
        $response.Headers["X-SimpleRedis-EvalDigest"] | Should -Be $script:KongEvalDigest
    }

    It "GET /redis/get-miss is redis:miss" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case "get-miss" -Header "X-SimpleRedis-GetMiss" -Expected "redis:miss"
    }

    It "GET /dragonfly/get-miss is redis:miss" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case "get-miss" -Header "X-SimpleRedis-GetMiss" -Expected "redis:miss"
    }

    It "GET /redis/msetex lands a positive TTL" {
        $response = Invoke-SimpleRedisCase -Engine redis -Case msetex
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Headers["X-SimpleRedis-MSetEX"] | Should -Be "ok"
        [int](@($response.Headers["X-SimpleRedis-MSetEX-TTL"])[0]) | Should -BeGreaterThan 0
    }

    It "GET /dragonfly/msetex lands a positive TTL" {
        $response = Invoke-SimpleRedisCase -Engine dragonfly -Case msetex
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Headers["X-SimpleRedis-MSetEX"] | Should -Be "ok"
        [int](@($response.Headers["X-SimpleRedis-MSetEX-TTL"])[0]) | Should -BeGreaterThan 0
    }

    It "GET /redis/msetexat returns the written value" {
        Assert-SimpleRedisHeaderCase -Engine redis -Case msetexat -Header "X-SimpleRedis-MSetEXAt" -Expected "ok"
    }

    It "GET /dragonfly/msetexat returns the written value" {
        Assert-SimpleRedisHeaderCase -Engine dragonfly -Case msetexat -Header "X-SimpleRedis-MSetEXAt" -Expected "ok"
    }

    It "GET /redis/drop double-applies Incr and Eval" {
        $response = Invoke-SimpleRedisCase -Engine redis -Case drop
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Headers["X-SimpleRedis-DropIncr"] | Should -Be "2"
        $response.Headers["X-SimpleRedis-DropIncrStored"] | Should -Be "2"
        $response.Headers["X-SimpleRedis-DropEval"] | Should -Be "6"
        $response.Headers["X-SimpleRedis-DropEvalStored"] | Should -Be "6"
    }

    It "GET /dragonfly/drop double-applies Incr and Eval" {
        $response = Invoke-SimpleRedisCase -Engine dragonfly -Case drop
        $response.StatusCode | Should -Be 200 -Because $response.Content
        $response.Headers["X-SimpleRedis-DropIncr"] | Should -Be "2"
        $response.Headers["X-SimpleRedis-DropIncrStored"] | Should -Be "2"
        $response.Headers["X-SimpleRedis-DropEval"] | Should -Be "6"
        $response.Headers["X-SimpleRedis-DropEvalStored"] | Should -Be "6"
    }

    It "GET /redis/eval proves EVALSHA miss then hit without stopping whoami-a or whoami-b" {
        Assert-EvalShaMissThenHit -Route "/redis/eval" -BackendHost "redis"
    }

    It "GET /dragonfly/eval proves EVALSHA miss then hit without stopping whoami-a or whoami-b" {
        Assert-EvalShaMissThenHit -Route "/dragonfly/eval" -BackendHost "dragonfly"
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

    It "two GET /redis/get return distinct own-values" {
        Assert-TwoDistinctOwnValues -Url "$script:BaseUrl/redis/get"
    }

    It "two GET /dragonfly/get return distinct own-values" {
        Assert-TwoDistinctOwnValues -Url "$script:BaseUrl/dragonfly/get"
    }

    It "GET /redis/recover recovers after CLIENT KILL of the Traefik client" {
        $warmup = Invoke-WebRequest -Uri "$script:BaseUrl/redis" -UseBasicParsing -TimeoutSec 10
        $warmup.StatusCode | Should -Be 200
        Stop-TraefikEngineClientForTest -Engine redis
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/redis/recover" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
        $response.Headers["X-SimpleRedis-Recover"] | Should -Be "ok"
    }

    It "GET /dragonfly/recover recovers after CLIENT KILL of the Traefik client" {
        $warmup = Invoke-WebRequest -Uri "$script:BaseUrl/dragonfly" -UseBasicParsing -TimeoutSec 10
        $warmup.StatusCode | Should -Be 200
        Stop-TraefikEngineClientForTest -Engine dragonfly
        $response = Invoke-WebRequest -Uri "$script:BaseUrl/dragonfly/recover" -UseBasicParsing -TimeoutSec 10
        $response.StatusCode | Should -Be 200
        $response.Headers["X-SimpleRedis-Recover"] | Should -Be "ok"
    }

    It "GET /redis/hold concurrent holds stay within default poolSize" {
        Assert-SimpleRedisLiveCap -Path "/redis/hold" -BackendHost "redis"
    }

    It "GET /dragonfly/hold concurrent holds stay within default poolSize" {
        Assert-SimpleRedisLiveCap -Path "/dragonfly/hold" -BackendHost "dragonfly"
    }
}
