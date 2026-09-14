#requires -Version 5.1
# Called only after explicit backup/restore consent.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
try {
    $package = Get-AppxPackage -Name OpenAI.Codex | Sort-Object { [version]$_.Version } -Descending | Select-Object -First 1
    $prefix = $null
    if ($null -ne $package) {
        if ([string]$package.PackageFamilyName -cne 'OpenAI.Codex_2p2nqsd0c76g0') { throw 'Unexpected app package family.' }
        $prefix = ([string]$package.InstallLocation).TrimEnd('\') + '\'
    }
    $session = [Diagnostics.Process]::GetCurrentProcess().SessionId
    $ours = @(); $external = @()
    foreach ($p in @(Get-Process)) {
        if ($p.Id -eq $PID -or $p.SessionId -ne $session) { continue }
        try { $path = [string]$p.Path } catch { $path = '' }
        if ($null -ne $prefix -and -not [string]::IsNullOrEmpty($path) -and $path.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { $ours += $p }
        elseif ($p.ProcessName -ieq 'codex') { $external += $p }
    }
    if ($external.Count -gt 0) { throw ('Codex CLI/IDE processes may be using .codex. Close them first (PIDs: {0}). No state was changed.' -f (($external | ForEach-Object { $_.Id }) -join ', ')) }
    if ($ours.Count -gt 0) {
        Write-Host '[>>] Closing this user''s ChatGPT/Codex desktop processes...'
        foreach ($p in $ours) { try { [void]$p.CloseMainWindow() } catch { } }
        Start-Sleep -Seconds 3
        foreach ($p in $ours) {
            if (-not $p.HasExited) { $p.Kill(); $p.WaitForExit(5000) | Out-Null }
            if (-not $p.HasExited) { throw 'Cannot close ChatGPT. Quit it and retry.' }
        }
    }
    Write-Host '[OK] Desktop app is closed. Keep CLI/IDE sessions closed until the operation finishes.'
}
catch { Write-Host $_.Exception.Message -ForegroundColor Red; exit 1 }
