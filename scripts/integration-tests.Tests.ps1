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

    function Invoke-RedisCli {
        param(
            [string]$BackendHost,
            [string[]]$CliArgs
        )
        $output = docker compose exec -T redis redis-cli -h $BackendHost --raw @CliArgs
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

    function Assert-SimpleRedisVerbHeaders {
        param($Response)
        $Response.Headers["X-SimpleRedis-Value"] | Should -Be "ok"
        $Response.Headers["X-SimpleRedis-MGet"] | Should -Be "ok"
        $Response.Headers["X-SimpleRedis-Del"] | Should -Be "ok"
        $Response.Headers["X-SimpleRedis-Incr"] | Should -Be "1"
        $Response.Headers["X-SimpleRedis-IncrBy"] | Should -Be "5"
        $Response.Headers["X-SimpleRedis-Expire"] | Should -Be "ok"
        $Response.Headers["X-SimpleRedis-ExpireAt"] | Should -Be "ok"
        $Response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
        $Response.Headers["X-SimpleRedis-EvalAgain"] | Should -Be "3"
        $Response.Headers["X-SimpleRedis-EvalDigest"] | Should -Be $script:KongEvalDigest
        $Response.Headers["X-SimpleRedis-DropIncr"] | Should -Be "2"
        $Response.Headers["X-SimpleRedis-DropIncrStored"] | Should -Be "2"
        $Response.Headers["X-SimpleRedis-DropEval"] | Should -Be "6"
        $Response.Headers["X-SimpleRedis-DropEvalStored"] | Should -Be "6"
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
        Assert-SimpleRedisVerbHeaders -Response $response
        (Get-ScriptExists -BackendHost $BackendHost -Digest $script:KongEvalDigest) | Should -Be "1"
        $again = Invoke-WebRequest -Uri "$script:BaseUrl$Route" -UseBasicParsing -TimeoutSec 10
        $again.StatusCode | Should -Be 200
        $again.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
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

    It "GET /redis proves EVALSHA miss then hit without stopping whoami-a or whoami-b" {
        Assert-EvalShaMissThenHit -Route "/redis" -BackendHost "redis"
    }

    It "GET /dragonfly proves EVALSHA miss then hit without stopping whoami-a or whoami-b" {
        Assert-EvalShaMissThenHit -Route "/dragonfly" -BackendHost "dragonfly"
    }
}
