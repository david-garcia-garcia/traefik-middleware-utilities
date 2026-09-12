. (Join-Path $PSScriptRoot "integration-tests.utils/Import.ps1")

Describe "reclaim Yaegi e2e" {
    It "Traefik API is reachable" {
        Assert-TraefikApiReachable
    }

    It "both routes succeed and share one reclaim incarnation" {
        $a = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/a" -UseBasicParsing -TimeoutSec 10
        $b = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/b" -UseBasicParsing -TimeoutSec 10
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
                $a = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/a" -UseBasicParsing -TimeoutSec 10
                $b = Invoke-WebRequest -Uri "$env:INTEGRATION_BASE_URL/b" -UseBasicParsing -TimeoutSec 10
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
