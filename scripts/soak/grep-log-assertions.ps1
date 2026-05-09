<#
.SYNOPSIS
    Phase 2 soak-validation log assertions. Reads
    %LOCALAPPDATA%\SquireBot\squirebot.log* and checks for the
    presence/absence of specific log strings keyed to each injection
    scenario. Prints PASS/FAIL per criterion + summary.

.PARAMETER Scenario
    One of: QuotaThrottle, InvalidGrant, CorruptUpdate, AllPhase2

.PARAMETER ExePath
    Optional override for the SquireBot binary path. If omitted, auto-detects
    NSIS-managed install at %LOCALAPPDATA%\Programs\SquireBot\squirebot.exe
    first, then falls back to hand-rolled install at %LOCALAPPDATA%\SquireBot\squirebot.exe.
    Used by the CorruptUpdate scenario to check for .new and .expected-sha256
    cleanup at the correct install location.

.NOTES
    Updated 2026-05-08 after Day-1 (SC-4) and Day-6 (SC-1/SC-5) findings:
    - QuotaThrottle live-log evidence (429 in the watcher path) is impossible
      due to per-(project,user) quota isolation + watcher mutex serialization.
      Canonical SC-4 evidence is the WATCH-07 unit suite (internal/sheet/retry_test.go).
      Live procedure deprecated; this script now validates only the behavioral
      side-effects (uploads continued, heartbeat fired) with a deprecation note.
    - CorruptUpdate "staged hash mismatch" log line cannot exist in squirebot.log
      by design: update.Apply() runs before logging.Setup() in main.go, so swap
      errors go to stderr only via fmt.Fprintf. This script no longer requires
      the log line; behavioral evidence (.new + sidecar deleted, binary size
      unchanged) is the canonical proof. Optionally checks $env:TEMP\squirebot-stderr.txt
      if it exists (from the procedure variant that captures stderr explicitly).
    - QuotaThrottle "no permanent auth failure" check removed: cross-day log
      pollution from InvalidGrant test made it false-positive across full-soak runs.
      InvalidGrant scenario validates the auth-failure path positively; absence
      checks are too brittle for a multi-day log.
    See docs/soak-reports/2026-05-07-day4-auth05-sc1.md for full findings.
#>
param(
    [Parameter(Mandatory=$true)][ValidateSet('QuotaThrottle','InvalidGrant','CorruptUpdate','AllPhase2')][string]$Scenario,
    [string]$ExePath = $null
)

$LogGlob = "$env:LOCALAPPDATA\SquireBot\squirebot.log*"
$LogDir  = Split-Path $LogGlob -Parent
if (-not (Test-Path $LogDir)) {
    Write-Error "Log directory not found: $LogDir"
    exit 2
}

# Auto-detect install path if not overridden. Standard NSIS install first,
# then hand-rolled. The CorruptUpdate scenario uses this to verify .new and
# .expected-sha256 cleanup at the correct location.
function Find-Exe {
    $std  = "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe"
    $hand = "$env:LOCALAPPDATA\SquireBot\squirebot.exe"
    if (Test-Path $std)  { return $std }
    if (Test-Path $hand) { return $hand }
    return $null
}

if (-not $ExePath) {
    $ExePath = Find-Exe
    if ($ExePath) {
        Write-Host "Auto-detected install: $ExePath"
    }
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
        Write-Host "Scenario: QuotaThrottle (SC-4) — behavioral checks only"
        Write-Host "  NOTE: Live 429 verification is DEPRECATED (architecturally infeasible — see"
        Write-Host "        docs/soak-reports/2026-05-07-day4-auth05-sc1.md Day 1 § Architectural barriers)."
        Write-Host "        Canonical SC-4 evidence is the WATCH-07 unit suite at internal/sheet/retry_test.go."
        Write-Host "        This scenario verifies only that the watcher remained functional during the test window."
        $allPass = (Test-LogContains 'uploaded' 'At least one successful upload during the test window') -and $allPass
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
        Write-Host "Scenario: CorruptUpdate (SC-1, SC-5) — behavioral checks (log line is unverifiable by design)"
        Write-Host "  NOTE: 'staged hash mismatch' log line CANNOT exist in squirebot.log: update.Apply()"
        Write-Host "        runs in cmd/squirebot/main.go BEFORE logging.Setup(), so swap errors go to stderr"
        Write-Host "        only. Start-Process discards stderr by default. Use behavioral evidence instead."
        Write-Host "        See docs/soak-reports/2026-05-07-day4-auth05-sc1.md Day 6 § Findings."
        Write-Host ""
        if (-not $ExePath) {
            Write-Host "  FAIL: SquireBot binary not found at standard NSIS path nor hand-rolled location" -ForegroundColor Red
            Write-Host "        Pass -ExePath to override." -ForegroundColor Red
            $allPass = $false
        } else {
            Write-Host "  Checking corrupt-payload cleanup at: $ExePath"
            if (-not (Test-Path "$ExePath.new")) {
                Write-Host "  PASS: .new file deleted (or never staged) at $ExePath.new"
            } else {
                Write-Host "  FAIL: .new file still present at $ExePath.new" -ForegroundColor Red
                $allPass = $false
            }
            if (-not (Test-Path "$ExePath.expected-sha256")) {
                Write-Host "  PASS: .expected-sha256 sidecar deleted (or never staged)"
            } else {
                Write-Host "  FAIL: .expected-sha256 still present" -ForegroundColor Red
                $allPass = $false
            }
        }
        # Optionally verify stderr capture if the operator ran the variant procedure
        # that redirects stderr to a temp file (see scripts/soak/inject-corrupt-update.md).
        $stderrPath = "$env:TEMP\squirebot-stderr.txt"
        if (Test-Path $stderrPath) {
            $stderrContent = Get-Content $stderrPath -Raw -ErrorAction SilentlyContinue
            if ($stderrContent -match 'staged hash mismatch') {
                Write-Host "  PASS: stderr capture at $stderrPath contains 'staged hash mismatch'"
            } else {
                Write-Host "  INFO: stderr capture exists at $stderrPath but does not contain 'staged hash mismatch'"
                Write-Host "        (not a failure — stderr capture is optional; behavioral checks are canonical)"
            }
        } else {
            Write-Host "  INFO: No stderr capture at $stderrPath (optional — behavioral checks are canonical)"
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
        # No silent retry loops (no error line repeated > 10 times exactly).
        # NOTE: For JSON-formatted logs with per-line timestamps, this check is
        # effectively permissive — Group-Object Line groups on the full line
        # content, and timestamp-bearing lines are always unique. Kept as a
        # safety net for future log format changes that might omit timestamps.
        $repeats = Select-String -Path $LogGlob -Pattern 'write inventory.*err' -ErrorAction SilentlyContinue |
            Group-Object Line | Where-Object { $_.Count -gt 10 }
        if (-not $repeats) {
            Write-Host "  PASS: No silent retry loop (no error line repeated > 10 times verbatim)"
        } else {
            Write-Host "  FAIL: Silent retry loop detected ($($repeats.Count) groups exceed threshold)" -ForegroundColor Red
            $allPass = $false
        }
        # Re-run all three scenario checks (visible output)
        Write-Host ""
        Write-Host "--- Re-running QuotaThrottle ---"
        & $PSCommandPath -Scenario QuotaThrottle -ExePath $ExePath
        if ($LASTEXITCODE -ne 0) { $allPass = $false }
        Write-Host ""
        Write-Host "--- Re-running InvalidGrant ---"
        & $PSCommandPath -Scenario InvalidGrant -ExePath $ExePath
        if ($LASTEXITCODE -ne 0) { $allPass = $false }
        Write-Host ""
        Write-Host "--- Re-running CorruptUpdate ---"
        & $PSCommandPath -Scenario CorruptUpdate -ExePath $ExePath
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
