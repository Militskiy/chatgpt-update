#requires -Version 5.1
# Visible, versioned self-update helper. Respects the caller's execution policy.
# Only the portable app's fixed file set is changed. No ChatGPT/project/state writes.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$PlanPath, [switch]$NoPause)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ok = $false; $plan = $null; $work = $null; $mutex = $null; $ready = $null
$changed = New-Object 'System.Collections.Generic.List[string]'
$old = @{}; $intended = @{}; $root = $null; $previous = $null
$names = @('VERSION','README.md','FILES.sha256','scripts/path.ps1','scripts/prepare-state.ps1','scripts/update-chatgpt.ps1','scripts/replace-updater.ps1','chatgpt-update.exe')
function Check-Path([string]$Path) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Links/junctions are not supported: $Path" }
}
function Hash([string]$Path) { return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() }
function Note([string]$Text) {
    Write-Host $Text
    if ($null -ne $plan) { Add-Content -LiteralPath ([string]$plan.log) -Value (('{0} {1}' -f [DateTime]::UtcNow.ToString('o'), $Text)) -Encoding UTF8 }
}
try {
    $plan = Get-Content -LiteralPath $PlanPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $target = [IO.Path]::GetFullPath([string]$plan.target)
    $root = [IO.Path]::GetDirectoryName($target)
    $candidate = [IO.Path]::GetFullPath([string]$plan.candidate)
    $previous = [IO.Path]::GetFullPath([string]$plan.previous)
    $work = [IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($PlanPath))
    if ([IO.Path]::GetFileName($target) -cne 'chatgpt-update.exe') { throw 'Unexpected executable name.' }
    if ([IO.Path]::GetDirectoryName($work) -ine $root -or [IO.Path]::GetFileName($work) -notlike '.chatgpt-update-stage-*') { throw 'Unexpected staging layout.' }
    if ($candidate -ine (Join-Path $work 'package')) { throw 'Unexpected package directory.' }
    if ([IO.Path]::GetDirectoryName($previous) -ine $root -or [IO.Path]::GetFileName($previous) -notlike '.chatgpt-update-previous-*') { throw 'Unexpected rollback directory.' }
    if (Test-Path -LiteralPath $previous) { throw 'Rollback directory already exists; refusing overwrite.' }
    if ([string]$plan.newVersion -notmatch '^\d+\.\d+\.\d+$' -or [string]$plan.lockName -notmatch '^Local\\Militskiy\.ChatGPTUpdater-[a-f0-9]{24}$') { throw 'Invalid replacement plan.' }
    foreach ($path in @($root,$work,$candidate,(Join-Path $candidate 'scripts'),(Join-Path $root 'scripts'))) { Check-Path $path }
    if (@($plan.files).Count -ne $names.Count) { throw 'Incorrect package file set.' }
    foreach ($record in $plan.files) {
        $name = [string]$record.path
        if ($names -cnotcontains $name -or $intended.ContainsKey($name) -or [string]$record.sha256 -notmatch '^[a-f0-9]{64}$') { throw 'Unexpected or duplicated component.' }
        $intended[$name] = [string]$record.sha256
        $source = Join-Path $candidate $name
        Check-Path $source
        if ((Get-Item -LiteralPath $source).Length -ne [long]$record.size -or (Hash $source) -cne $intended[$name]) { throw "Staged component changed: $name" }
    }
    if (@(Get-ChildItem -LiteralPath $candidate -Recurse -File -Force).Count -ne $names.Count) { throw 'Unexpected files in staged package.' }
    Note '[>>] Waiting for the previous updater to exit...'
    $parent = Get-Process -Id ([int]$plan.parentPid) -ErrorAction SilentlyContinue
    if ($null -ne $parent -and -not $parent.WaitForExit(60000)) { throw 'Previous updater is still running.' }
    $created = $false
    $mutex = New-Object Threading.Mutex($false, ([string]$plan.lockName), [ref]$created)
    if (-not $created) { throw 'Another updater operation is running. No files were changed.' }
    $candidateExe = Join-Path $candidate 'chatgpt-update.exe'
    $reported = (& $candidateExe --verify-package | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reported -cne [string]$plan.newVersion) { throw 'New EXE/component verification failed.' }
    # Snapshot every current component before the first replacement. Missing
    # optional documentation is recorded so rollback restores the original set.
    New-Item -ItemType Directory -Path $previous | Out-Null
    foreach ($name in $names) {
        $dest = Join-Path $root $name
        if (Test-Path -LiteralPath $dest) {
            Check-Path $dest
            $old[$name] = Hash $dest
            $saved = Join-Path $previous $name
            New-Item -ItemType Directory -Path (Split-Path $saved -Parent) -Force | Out-Null
            Copy-Item -LiteralPath $dest -Destination $saved
            if ((Hash $saved) -cne $old[$name]) { throw "Cannot preserve component: $name" }
        } else { $old[$name] = $null }
    }
    Note '[>>] Replacing the complete portable package; previous files are retained...'
    foreach ($name in $names) {
        $dest = Join-Path $root $name
        $ready = $dest + '.ready-' + [Guid]::NewGuid().ToString('N')
        Copy-Item -LiteralPath (Join-Path $candidate $name) -Destination $ready
        if ((Hash $ready) -cne $intended[$name]) { throw "Prepared component hash mismatch: $name" }
        if (Test-Path -LiteralPath $dest) {
            Check-Path $dest
            if ($null -eq $old[$name] -or (Hash $dest) -cne $old[$name]) { throw "Current component changed concurrently: $name" }
        } elseif ($null -ne $old[$name]) { throw "Current component disappeared: $name" }
        Move-Item -LiteralPath $ready -Destination $dest -Force
        $ready = $null
        $changed.Add($name)
        if ((Hash $dest) -cne $intended[$name]) { throw "Installed component mismatch: $name" }
    }
    $reported = (& $target --verify-package | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reported -cne [string]$plan.newVersion) { throw 'Installed package verification failed.' }
    Note ('[OK] Updater package is now {0}.' -f $plan.newVersion)
    Note ('Previous package retained: {0}' -f $previous)
    Note 'Run chatgpt-update again to open the menu. ChatGPT and .codex were not changed.'
    $ok = $true
} catch {
    Write-Host ('[FAIL] {0}' -f $_.Exception.Message) -ForegroundColor Red
    if ($changed.Count -gt 0) {
        for ($i = $changed.Count - 1; $i -ge 0; $i--) {
            $name = $changed[$i]; $dest = Join-Path $root $name
            try {
                if (Test-Path -LiteralPath $dest) {
                    Check-Path $dest
                    if ((Hash $dest) -cne $intended[$name]) { throw 'Target was changed externally; manual rollback required.' }
                }
                if ($null -ne $old[$name]) {
                    $saved = Join-Path $previous $name
                    if ((Hash $saved) -cne $old[$name]) { throw 'Rollback copy verification failed.' }
                    Copy-Item -LiteralPath $saved -Destination $dest -Force
                    if ((Hash $dest) -cne $old[$name]) { throw 'Restored component verification failed.' }
                } elseif (Test-Path -LiteralPath $dest) { Remove-Item -LiteralPath $dest -Force }
                Write-Host ("[OK] Rolled back {0}" -f $name)
            } catch { Write-Host ("Manual recovery required for {0}: {1}. Preserved files: {2}" -f $name,$_.Exception.Message,$previous) -ForegroundColor Yellow }
        }
    }
    if ($null -ne $plan) { Write-Host ('Log: {0}' -f $plan.log) }
} finally {
    if ($null -ne $mutex) { $mutex.Dispose() }
    if ($null -ne $ready -and (Test-Path -LiteralPath $ready)) { Remove-Item -LiteralPath $ready -Force -ErrorAction SilentlyContinue }
    if ($ok -and $null -ne $work) { Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue }
    if (-not $NoPause) { [void](Read-Host 'Press Enter to close this window') }
}
if (-not $ok) { exit 1 }
