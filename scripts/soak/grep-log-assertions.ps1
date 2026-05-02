<#
.SYNOPSIS
    Phase 2 soak-validation log assertions. Reads
    %LOCALAPPDATA%\SquireBot\squirebot.log* and checks for the
    presence/absence of specific log strings keyed to each injection
    scenario. Prints PASS/FAIL per criterion + summary.

.PARAMETER Scenario
    One of: QuotaThrottle, InvalidGrant, CorruptUpdate, AllPhase2
#>
param([Parameter(Mandatory=$true)][ValidateSet('QuotaThrottle','InvalidGrant','CorruptUpdate','AllPhase2')][string]$Scenario)

$LogGlob = "$env:LOCALAPPDATA\SquireBot\squirebot.log*"
$LogDir  = Split-Path $LogGlob -Parent
if (-not (Test-Path $LogDir)) {
    Write-Error "Log directory not found: $LogDir"
    exit 2
}

function Test-LogContains {
    param([string]$Pattern, [string]$Description)
    $hits = Select-String -Path $LogGlob -Pattern $Pattern -ErrorAction SilentlyContinue
    if ($hits) {
        Write-Host "  PASS: $Description (matches: $($hits.Count))"
        return $true
    } else {
        Write-Host "  FAIL: $Description (no match for /$Pattern/)" -ForegroundColor Red
        return $false
    }
}

function Test-LogDoesNotContain {
    param([string]$Pattern, [string]$Description)
    $hits = Select-String -Path $LogGlob -Pattern $Pattern -ErrorAction SilentlyContinue
    if (-not $hits) {
        Write-Host "  PASS: $Description"
        return $true
    } else {
        Write-Host "  FAIL: $Description (unexpected matches: $($hits.Count))" -ForegroundColor Red
        return $false
    }
}

$allPass = $true

switch ($Scenario) {
    'QuotaThrottle' {
        Write-Host "Scenario: QuotaThrottle (SC-4)"
        $allPass = (Test-LogContains '429|userRateLimitExceeded|rateLimitExceeded' 'At least one 429 / userRateLimitExceeded') -and $allPass
        $allPass = (Test-LogContains 'uploaded' 'At least one successful upload after the throttle') -and $allPass
        $allPass = (Test-LogDoesNotContain 'permanent auth failure' 'No permanent auth failure during throttle') -and $allPass
        $allPass = (Test-LogContains 'heartbeat written' 'Heartbeat fired during the test window') -and $allPass
    }
    'InvalidGrant' {
        Write-Host "Scenario: InvalidGrant (SC-1, AUTH-05)"
        $allPass = (Test-LogContains 'permanent auth failure' 'permanent auth failure log line') -and $allPass
        $allPass = (Test-LogContains 'auth suspended' 'auth suspended log line') -and $allPass
        $allPass = (Test-LogContains 'Reauthorize start' 'Reauthorize start log line') -and $allPass
        $allPass = (Test-LogContains 'Reauthorize complete' 'Reauthorize complete log line') -and $allPass
        $allPass = (Test-LogContains 'uploaded' 'At least one upload AFTER reauth') -and $allPass
    }
    'CorruptUpdate' {
        Write-Host "Scenario: CorruptUpdate (SC-1, SC-5)"
        $allPass = (Test-LogContains 'staged hash mismatch' 'staged hash mismatch log line') -and $allPass
        $exe = "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe"
        if (-not (Test-Path "$exe.new")) {
            Write-Host "  PASS: .new file deleted after failed swap"
        } else {
            Write-Host "  FAIL: .new file still present after failed swap" -ForegroundColor Red
            $allPass = $false
        }
        if (-not (Test-Path "$exe.expected-sha256")) {
            Write-Host "  PASS: .expected-sha256 sidecar deleted after failed swap"
        } else {
            Write-Host "  FAIL: .expected-sha256 still present after failed swap" -ForegroundColor Red
            $allPass = $false
        }
    }
    'AllPhase2' {
        Write-Host "Scenario: AllPhase2 (full sweep)"
        # Heartbeat count
        $hbCount = (Select-String -Path $LogGlob -Pattern 'heartbeat written' -ErrorAction SilentlyContinue | Measure-Object).Count
        if ($hbCount -ge 5) {
            Write-Host "  PASS: heartbeat fired $hbCount times (>= 5 expected over 7 days)"
        } else {
            Write-Host "  FAIL: heartbeat fired only $hbCount times (need >= 5)" -ForegroundColor Red
            $allPass = $false
        }
        # No silent retry loops (no error line repeated > 10 times)
        $repeats = Select-String -Path $LogGlob -Pattern 'write inventory.*err' -ErrorAction SilentlyContinue |
            Group-Object Line | Where-Object { $_.Count -gt 10 }
        if (-not $repeats) {
            Write-Host "  PASS: No silent retry loop (no error line repeated > 10 times)"
        } else {
            Write-Host "  FAIL: Silent retry loop detected ($($repeats.Count) groups exceed threshold)" -ForegroundColor Red
            $allPass = $false
        }
        # Re-run all three scenario checks (visible output)
        Write-Host ""
        Write-Host "--- Re-running QuotaThrottle ---"
        & $PSCommandPath -Scenario QuotaThrottle
        if ($LASTEXITCODE -ne 0) { $allPass = $false }
        Write-Host ""
        Write-Host "--- Re-running InvalidGrant ---"
        & $PSCommandPath -Scenario InvalidGrant
        if ($LASTEXITCODE -ne 0) { $allPass = $false }
        Write-Host ""
        Write-Host "--- Re-running CorruptUpdate ---"
        & $PSCommandPath -Scenario CorruptUpdate
        if ($LASTEXITCODE -ne 0) { $allPass = $false }
    }
}

Write-Host ""
if ($allPass) {
    Write-Host "OVERALL: PASS $Scenario" -ForegroundColor Green
    exit 0
} else {
    Write-Host "OVERALL: FAIL $Scenario" -ForegroundColor Red
    exit 1
}
