# SimpleRedis HTTP probe helpers for Pester. Dot-sourced via Import.ps1.

$script:SimpleRedisEngines = @(
    @{ Engine = "redis"; BackendHost = "redis" }
    @{ Engine = "dragonfly"; BackendHost = "dragonfly" }
)

$script:KongIncrbyExpireatScript = @'
local exists = redis.call("exists", KEYS[1])
local value = redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
return value
'@
$script:TtlScript = "return redis.call('TTL', KEYS[1])"
$script:TimeWaitHoldScript = @'
local start = redis.call("TIME")
local startSec = tonumber(start[1])
local startUsec = tonumber(start[2])
local need = tonumber(ARGV[1])
while true do
  local now = redis.call("TIME")
  local elapsed = (tonumber(now[1]) - startSec) * 1000000 + (tonumber(now[2]) - startUsec)
  if elapsed >= need then
    break
  end
end
return 1
'@
$script:KongIncrbyExpireatScript = $script:KongIncrbyExpireatScript.Replace("`r`n", "`n")
$script:TimeWaitHoldScript = $script:TimeWaitHoldScript.Replace("`r`n", "`n")

# Get-Sha1Hex is lowercase hex SHA-1 of Text (Redis EVALSHA digest).
function Get-Sha1Hex {
    param([string]$Text)
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Text)
    $hash = [System.Security.Cryptography.SHA1]::Create().ComputeHash($bytes)
    return -join ($hash | ForEach-Object { $_.ToString("x2") })
}

$script:KongEvalDigest = Get-Sha1Hex $script:KongIncrbyExpireatScript
$script:TtlEvalDigest = Get-Sha1Hex $script:TtlScript
$script:TimeWaitHoldDigest = Get-Sha1Hex $script:TimeWaitHoldScript

# New-SimpleRedisKey is a unique key name for one Pester request.
function New-SimpleRedisKey {
    return "srp:$([DateTime]::UtcNow.Ticks)-$([guid]::NewGuid().ToString('N').Substring(0, 8))"
}

# Invoke-SimpleRedis calls /<Engine>/<Verb> with query args; Body makes the request POST.
function Invoke-SimpleRedis {
    param(
        [Parameter(Mandatory)]
        [string]$Engine,
        [Parameter(Mandatory)]
        [string]$Verb,
        [string[]]$Key,
        [string[]]$Arg,
        [string]$Ex,
        [string]$At,
        [string]$Delta,
        [string]$Digest,
        [switch]$Drop,
        [string]$Body,
        [int]$TimeoutSec = 10
    )
    $parts = @()
    foreach ($name in @($Key)) {
        if ($null -ne $name -and $name -ne "") {
            $parts += "key=$([uri]::EscapeDataString($name))"
        }
    }
    foreach ($value in @($Arg)) {
        if ($null -ne $value) {
            $parts += "arg=$([uri]::EscapeDataString($value))"
        }
    }
    if ($Ex) { $parts += "ex=$([uri]::EscapeDataString($Ex))" }
    if ($At) { $parts += "at=$([uri]::EscapeDataString($At))" }
    if ($Delta) { $parts += "delta=$([uri]::EscapeDataString($Delta))" }
    if ($Digest) { $parts += "digest=$([uri]::EscapeDataString($Digest))" }
    if ($Drop) { $parts += "drop=1" }
    $uri = "$script:BaseUrl/$Engine/$Verb"
    if ($parts.Count -gt 0) {
        $uri = "$uri`?$($parts -join '&')"
    }
    $invoke = @{
        Uri                = $uri
        UseBasicParsing    = $true
        TimeoutSec         = $TimeoutSec
        SkipHttpErrorCheck = $true
    }
    if ($PSBoundParameters.ContainsKey("Body")) {
        $invoke.Method = "POST"
        $invoke.Body = $Body
        $invoke.ContentType = "text/plain; charset=utf-8"
    }
    Invoke-WebRequest @invoke
}

# Invoke-RedisCli runs redis-cli -h BackendHost inside compose redis.
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

# Clear-ScriptCache runs SCRIPT FLUSH on BackendHost.
function Clear-ScriptCache {
    param([string]$BackendHost)
    Invoke-RedisCli -BackendHost $BackendHost -CliArgs @("SCRIPT", "FLUSH") | Out-Null
}

# Get-ScriptExists is SCRIPT EXISTS of Digest on BackendHost.
function Get-ScriptExists {
    param(
        [string]$BackendHost,
        [string]$Digest
    )
    Invoke-RedisCli -BackendHost $BackendHost -CliArgs @("SCRIPT", "EXISTS", $Digest)
}

# Assert-EvalShaMissThenHit is SCRIPT FLUSH, EXISTS 0, Eval 3, EXISTS 1, Eval 3 on a new key.
function Assert-EvalShaMissThenHit {
    param(
        [string]$Engine,
        [string]$BackendHost
    )
    $expireUnix = [DateTimeOffset]::UtcNow.AddSeconds(60).ToUnixTimeSeconds().ToString()
    Clear-ScriptCache -BackendHost $BackendHost
    (Get-ScriptExists -BackendHost $BackendHost -Digest $script:KongEvalDigest) | Should -Be "0"
    $response = Invoke-SimpleRedis -Engine $Engine -Verb eval -Key (New-SimpleRedisKey) -Arg @("3", $expireUnix) -Digest $script:KongEvalDigest -Body $script:KongIncrbyExpireatScript
    $response.StatusCode | Should -Be 200 -Because $response.Content
    $response.Content.Trim() | Should -Be "3"
    (Get-ScriptExists -BackendHost $BackendHost -Digest $script:KongEvalDigest) | Should -Be "1"
    $again = Invoke-SimpleRedis -Engine $Engine -Verb eval -Key (New-SimpleRedisKey) -Arg @("3", $expireUnix) -Digest $script:KongEvalDigest -Body $script:KongIncrbyExpireatScript
    $again.StatusCode | Should -Be 200 -Because $again.Content
    $again.Content.Trim() | Should -Be "3"
}

# Assert-TwoDistinctHealthValues is two GETs of Url with distinct srp: tokens.
function Assert-TwoDistinctHealthValues {
    param([string]$Url)
    $first = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 15 -SkipHttpErrorCheck
    $second = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 15 -SkipHttpErrorCheck
    $first.StatusCode | Should -Be 200 -Because $first.Content
    $second.StatusCode | Should -Be 200 -Because $second.Content
    $a = $first.Content.Trim()
    $b = $second.Content.Trim()
    $a | Should -Match '^srp:\d+$'
    $b | Should -Match '^srp:\d+$'
    $a | Should -Not -Be $b
}

# Wait-BackendPing starts redis and BackendHost and waits for PONG.
function Wait-BackendPing {
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

# Start-NetnsReader is a sidecar on BackendHost's netns so Pester can read /proc/net/tcp (Redis uses the engine container itself).
function Start-NetnsReader {
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

# Read-BackendTcp is /proc/net/tcp and tcp6 from Redis or the Dragonfly sidecar.
function Read-BackendTcp {
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

# Count-Established6379 is how many ESTABLISHED rows target port 6379.
function Count-Established6379 {
    param($TcpDump)
    return [regex]::Matches("$TcpDump", ':18[Ee][Bb]\s+\S+\s+01\s').Count
}

# Assert-SimpleRedisLiveCap posts overlapping TIME-wait Evals on Path and checks poolSize plus a 502 waiter.
function Assert-SimpleRedisLiveCap {
    param(
        [string]$Path,
        [string]$BackendHost
    )
    Wait-BackendPing -BackendHost $BackendHost
    $sidecar = Start-NetnsReader -BackendHost $BackendHost
    $holdUrl = "$script:BaseUrl$Path`?arg=500000&digest=$([uri]::EscapeDataString($script:TimeWaitHoldDigest))"
    $http = [System.Net.Http.HttpClient]::new()
    $http.Timeout = [TimeSpan]::FromSeconds(15)
    try {
        # Default liveCap is 8; fill poolSize then the waiter is redis:unreachable.
        $holds = 1..8 | ForEach-Object {
            $http.PostAsync($holdUrl, ([System.Net.Http.StringContent]::new($script:TimeWaitHoldScript)))
        }
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
        $waiter = $http.PostAsync($holdUrl, ([System.Net.Http.StringContent]::new($script:TimeWaitHoldScript))).GetAwaiter().GetResult()
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
