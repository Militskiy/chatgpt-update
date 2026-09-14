#requires -Version 5.1
# Separate console helper. Never changes ChatGPT, projects or backup state.
[CmdletBinding()]
param([Parameter(Mandatory=$true)] [string]$PlanPath, [switch]$NoPause)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$oldMoved = $false; $plan = $null; $ok = $false; $work = $null; $ready = $null
try {
    $plan = Get-Content -LiteralPath $PlanPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $target = [IO.Path]::GetFullPath([string]$plan.target)
    $candidate = [IO.Path]::GetFullPath([string]$plan.candidate)
    # .NET Framework can expand an 8.3 directory name (e.g. RUNNER~1).
    # Normalize all compared paths consistently, including a not-yet-created rollback file.
    $previous = [IO.Path]::GetFullPath([string]$plan.previous)
    $plan.target = $target
    $plan.candidate = $candidate
    $plan.previous = $previous
    $work = [IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($PlanPath))
    if ([IO.Path]::GetDirectoryName($candidate) -cne $work -or [IO.Path]::GetDirectoryName($work) -ine [IO.Path]::GetDirectoryName($target)) { throw 'Unexpected self-update staging layout.' }
    if ([IO.Path]::GetFileName($candidate) -cne 'chatgpt-update.exe' -or [IO.Path]::GetExtension($target) -ine '.exe') { throw 'Unexpected executable path.' }
    if ([string]$plan.sha256 -notmatch '^[a-f0-9]{64}$' -or [string]$plan.newVersion -notmatch '^\d+\.\d+\.\d+$') { throw 'Invalid replacement plan.' }
    if (-not $previous.StartsWith($target + '.', [StringComparison]::OrdinalIgnoreCase) -or -not $previous.EndsWith('.previous', [StringComparison]::OrdinalIgnoreCase)) { throw ('Invalid rollback path. Target={0}; rollback={1}' -f $target, $previous) }
    function Note([string]$message) {
        Write-Host $message
        Add-Content -LiteralPath ([string]$plan.log) -Value ('{0} {1}' -f ([DateTime]::UtcNow.ToString('o')), $message) -Encoding UTF8
    }
    Note '[>>] Waiting for the previous updater process to exit...'
    $parent = Get-Process -Id ([int]$plan.parentPid) -ErrorAction SilentlyContinue
    if ($null -ne $parent) { if (-not $parent.WaitForExit(60000)) { throw 'Old updater is still running. No executable was replaced.' } }
    if ((Get-FileHash -LiteralPath $candidate -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$plan.sha256) { throw 'Staged executable checksum changed. Aborting.' }
    $reported = (& $candidate --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reported -cne [string]$plan.newVersion) { throw 'New executable failed its version probe.' }
    if (Test-Path -LiteralPath ([string]$plan.previous)) { throw 'Rollback path already exists; nothing was overwritten.' }
    # Prepare a complete sibling first; never overwrite a running EXE in place.
    $ready = $target + '.ready-' + [Guid]::NewGuid().ToString('N')
    Copy-Item -LiteralPath $candidate -Destination $ready
    if ((Get-FileHash -LiteralPath $ready -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$plan.sha256) { throw 'Prepared executable verification failed.' }
    Note '[>>] Preserving the previous executable...'
    Move-Item -LiteralPath $target -Destination ([string]$plan.previous)
    $oldMoved = $true
    Move-Item -LiteralPath $ready -Destination $target
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$plan.sha256) { throw 'Installed executable verification failed.' }
    $reported = (& $target --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reported -cne [string]$plan.newVersion) { throw 'Installed executable failed its version probe.' }
    Note ('[OK] Updater is now {0}.' -f $plan.newVersion)
    Note ('Previous executable retained: {0}' -f $plan.previous)
    Note 'ChatGPT and its data were not changed. Run chatgpt-update again to open the menu.'
    $ok = $true
}
catch {
    $message = $_.Exception.Message
    Write-Host ('[FAIL] {0}' -f $message) -ForegroundColor Red
    if ($null -ne $plan) {
        try { Add-Content -LiteralPath ([string]$plan.log) -Value ('FAIL: ' + $message) -Encoding UTF8 } catch { }
        if ($oldMoved) {
            try {
                if (Test-Path -LiteralPath ([string]$plan.target)) {
                    $currentHash = (Get-FileHash -LiteralPath ([string]$plan.target) -Algorithm SHA256).Hash.ToLowerInvariant()
                    if ($currentHash -cne [string]$plan.sha256) { throw 'Target differs from staged release; manual rollback required.' }
                    Remove-Item -LiteralPath ([string]$plan.target) -Force
                }
                Move-Item -LiteralPath ([string]$plan.previous) -Destination ([string]$plan.target)
                Write-Host '[OK] Restored the previous executable.'
            } catch { Write-Host ('Original executable retained at {0}. Rollback issue: {1}' -f $plan.previous, $_.Exception.Message) -ForegroundColor Yellow }
        }
        Write-Host ('Log: {0}' -f $plan.log)
    }
}
finally {
    if ($null -ne $ready -and (Test-Path -LiteralPath $ready)) { Remove-Item -LiteralPath $ready -Force -ErrorAction SilentlyContinue }
    if ($ok -and $null -ne $work) { Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue }
    if (-not $NoPause) { [void](Read-Host 'Press Enter to close this update window') }
    if ((Split-Path -Leaf $PSScriptRoot) -like 'ChatGPTUpdater-script-*') { Remove-Item -LiteralPath $PSScriptRoot -Recurse -Force -ErrorAction SilentlyContinue }
}
if (-not $ok) { exit 1 }
