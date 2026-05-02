<#
.SYNOPSIS
    Best-effort snapshot of SquireBot tray health by parsing the most
    recent state-transition log line. The systray library has no
    inspection API on Windows; we infer state from the slog'd
    transitions.

.OUTPUT
    Prints "GREEN", "RED", or "UNKNOWN" plus the timestamp of the
    inferred transition. Exits 0 on GREEN, 1 on RED, 2 on UNKNOWN.

.NOTES
    Tray state is set via t.SetIconHealth() and t.SetStatus() which
    update the tooltip but do NOT directly slog. The signals we CAN
    grep for in squirebot.log are the slog-emitted lines from the
    refresh / reauth / heartbeat / update paths:
      - "permanent auth failure" -> red (AUTH-05 trigger)
      - "auth suspended"          -> red (suspension active)
      - "Reauthorize complete"    -> green (recovery)
      - "heartbeat written"       -> green (writes are working)
    Most recent matching line wins. This is a heuristic; the true
    source-of-truth is the live tray icon, which the operator should
    visually verify per the runbook.
#>
$LogGlob = "$env:LOCALAPPDATA\SquireBot\squirebot.log*"

$greenPatterns = @('Reauthorize complete', 'heartbeat written', 'auto-update applied')
$redPatterns   = @('permanent auth failure', 'auth suspended')

$latestGreen = Select-String -Path $LogGlob -Pattern ($greenPatterns -join '|') -ErrorAction SilentlyContinue | Select-Object -Last 1
$latestRed   = Select-String -Path $LogGlob -Pattern ($redPatterns   -join '|') -ErrorAction SilentlyContinue | Select-Object -Last 1

function Get-LineTimestamp {
    param($line)
    if ($line -and $line.Line -match '(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})') {
        return $Matches[1]
    }
    return ""
}

if (-not $latestGreen -and -not $latestRed) {
    Write-Host "UNKNOWN - no matching log lines"
    exit 2
}
if (-not $latestRed) {
    Write-Host "GREEN - last green: $(Get-LineTimestamp $latestGreen)"
    exit 0
}
if (-not $latestGreen) {
    Write-Host "RED - last red: $(Get-LineTimestamp $latestRed)"
    exit 1
}
# Both present -- whichever appears later in the log file wins.
if ($latestGreen.LineNumber -gt $latestRed.LineNumber) {
    Write-Host "GREEN - last green: $(Get-LineTimestamp $latestGreen) (after last red: $(Get-LineTimestamp $latestRed))"
    exit 0
} else {
    Write-Host "RED - last red: $(Get-LineTimestamp $latestRed) (last green was earlier: $(Get-LineTimestamp $latestGreen))"
    exit 1
}
