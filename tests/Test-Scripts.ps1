#requires -Version 5.1
# Only temporary fixtures; no ChatGPT deployment, user PATH or real state changes.
[CmdletBinding()]
param([string]$Executable = (Join-Path $PSScriptRoot '../dist/portable/chatgpt-update.exe'))
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'; $global:LASTEXITCODE = 0
$root = Split-Path $PSScriptRoot -Parent
foreach ($file in @(Get-ChildItem (Join-Path $root 'scripts') -Filter *.ps1)) {
    $tokens=$null; $errors=$null
    $null=[Management.Automation.Language.Parser]::ParseFile($file.FullName,[ref]$tokens,[ref]$errors)
    if ($errors.Count) { throw ($errors | Out-String) }
    Write-Host ('PASS: parse {0}' -f $file.Name)
}
& (Join-Path $PSScriptRoot 'Test-UpdateEngine.ps1')
if ($LASTEXITCODE) { throw 'Engine tests failed.' }
& (Join-Path $PSScriptRoot 'Test-Ui.ps1')
if ($LASTEXITCODE) { throw 'UI tests failed.' }
$expected=(Get-Content (Join-Path $root 'VERSION') -Raw).Trim()
if ((& $Executable --version | Out-String).Trim() -cne $expected -or $LASTEXITCODE) { throw 'EXE version test failed.' }
if ((& $Executable --verify-package | Out-String).Trim() -cne $expected -or $LASTEXITCODE) { throw 'Sidecar package test failed.' }
& $Executable --help
if ($LASTEXITCODE) { throw 'EXE help test failed.' }
$temp=Join-Path $env:TEMP ('ChatGPTUpdater-selftest-'+[Guid]::NewGuid().ToString('N'))
try {
    New-Item -ItemType Directory $temp | Out-Null
    # The runner TEMP value may use RUNNER~1, while Get-ChildItem expands it.
    # Normalize first so relative paths are not cut using a short-path length.
    $temp=(Get-Item -LiteralPath $temp).FullName
    $portable=Split-Path $Executable -Parent
    Copy-Item (Join-Path $portable '*') $temp -Recurse
    $target=Join-Path $temp 'chatgpt-update.exe'
    $work=Join-Path $temp '.chatgpt-update-stage-test'
    $candidate=Join-Path $work 'package'
    New-Item -ItemType Directory $candidate -Force | Out-Null
    Copy-Item (Join-Path $portable '*') $candidate -Recurse
    $candidate=(Get-Item -LiteralPath $candidate).FullName
    $records=@(Get-ChildItem $candidate -Recurse -File | ForEach-Object {
        @{ path=$_.FullName.Substring($candidate.Length+1).Replace('\','/'); size=$_.Length; sha256=(Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
    })
    $plan=@{parentPid=2147483000;target=$target;candidate=$candidate;newVersion=$expected;previous=(Join-Path $temp '.chatgpt-update-previous-test');log=(Join-Path $temp 'replace.log');lockName='';files=$records}
    $plan.lockName='Local\Militskiy.ChatGPTUpdater-'+[Guid]::NewGuid().ToString('N').Substring(0,24)
    $planPath=Join-Path $work 'replacement.json'
    $plan | ConvertTo-Json -Depth 5 | Set-Content $planPath -Encoding UTF8
    # Test rejecting a changed stage before any existing package writes.
    Add-Content (Join-Path $candidate 'scripts/path.ps1') '# changed fixture only'
    $hostExe=[Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
    & $hostExe -NoProfile -File (Join-Path $temp 'scripts/replace-updater.ps1') -PlanPath $planPath -NoPause
    if ($LASTEXITCODE -eq 0) { throw 'Changed staging package was accepted.' }
    if (Test-Path $plan.previous) { throw 'Old package touched before stage validation.' }
    $statusFile=Join-Path $temp '.chatgpt-update-status.json'
    $state=Get-Content -LiteralPath $statusFile -Raw | ConvertFrom-Json
    if ($state.phase -cne 'failed' -or $state.message -notmatch 'Staged component changed') { throw 'Startup validation failure not persisted.' }
    if ((Get-Content -LiteralPath $plan.log -Raw) -notmatch '\[FAIL\]') { throw 'Helper failure not recorded in log.' }
    Copy-Item (Join-Path $portable 'scripts/path.ps1') (Join-Path $candidate 'scripts/path.ps1') -Force
    # Force a late replacement failure while preserving the previous EXE.
    # A changed old README makes rollback observable instead of comparing
    # byte-identical copies. Only temporary fixtures are touched.
    $oldReadme = 'previous locally customized documentation'
    [IO.File]::WriteAllText((Join-Path $temp 'README.md'), $oldReadme)
    $lock = [IO.File]::Open($target, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    try {
        & $hostExe -NoProfile -File (Join-Path $temp 'scripts/replace-updater.ps1') -PlanPath $planPath -NoPause
        if ($LASTEXITCODE -eq 0) { throw 'Locked target replacement unexpectedly succeeded.' }
    } finally { $lock.Dispose() }
    if (-not (Test-Path -LiteralPath (Join-Path $plan.previous 'README.md'))) { throw 'Late-failure test did not reach the replacement phase.' }
    if ([IO.File]::ReadAllText((Join-Path $temp 'README.md')) -cne $oldReadme) { throw 'Partial replacement failed to restore original README.' }
    if ((& $target --verify-package | Out-String).Trim() -cne $expected -or $LASTEXITCODE) { throw 'Rollback did not preserve valid original scripts/EXE.' }
    Write-Host 'PASS: late failure rolls back changed components without deleting original EXE'
    $plan.previous=Join-Path $temp '.chatgpt-update-previous-success'
    $plan | ConvertTo-Json -Depth 5 | Set-Content $planPath -Encoding UTF8
    & $hostExe -NoProfile -File (Join-Path $temp 'scripts/replace-updater.ps1') -PlanPath $planPath -NoPause
    if ($LASTEXITCODE) { throw 'Complete-package replacement smoke test failed.' }
    foreach ($record in $records) {
        if (-not (Test-Path -LiteralPath (Join-Path $plan.previous $record.path))) { throw 'An old component was not preserved.' }
        if ((Get-FileHash (Join-Path $temp $record.path) -Algorithm SHA256).Hash.ToLowerInvariant() -cne $record.sha256) { throw 'Replacement component mismatch.' }
    }
    if ((& $target --verify-package | Out-String).Trim() -cne $expected -or $LASTEXITCODE) { throw 'Replaced package invalid.' }
    $state=Get-Content -LiteralPath $statusFile -Raw | ConvertFrom-Json
    if ($state.phase -cne 'success' -or $state.version -cne $expected) { throw 'Successful replacement status not recorded.' }
    Write-Host 'PASS: tampered-stage rejection, persistent results and complete-package replacement with preserved old files'
    # Interactive helper mode: no -NoPause, and the same -NoExit that keeps
    # pre-script failures readable in the production launcher. There must be
    # no Read-Host on success and no shell left behind after completion.
    New-Item -ItemType Directory $candidate -Force | Out-Null
    Copy-Item (Join-Path $portable '*') $candidate -Recurse
    $plan.previous=Join-Path $temp '.chatgpt-update-previous-auto-close'
    $plan | ConvertTo-Json -Depth 5 | Set-Content $planPath -Encoding UTF8
    $processInfo=New-Object Diagnostics.ProcessStartInfo
    $processInfo.FileName=$hostExe
    $processInfo.Arguments='-NoLogo -NoProfile -NoExit -File "{0}" -PlanPath "{1}"' -f (Join-Path $temp 'scripts/replace-updater.ps1'),$planPath
    $processInfo.UseShellExecute=$false
    $processInfo.RedirectStandardInput=$true
    $child=[Diagnostics.Process]::Start($processInfo)
    try {
        # Keep stdin OPEN but send no input; a Read-Host would hang this test.
        if (-not $child.WaitForExit(10000)) { throw 'Success left the helper waiting for input or at a shell prompt.' }
        if ($child.ExitCode -ne 0) { throw 'Auto-closing helper returned an error.' }
        $state=Get-Content -LiteralPath $statusFile -Raw | ConvertFrom-Json
        if ($state.phase -cne 'success' -or $state.version -cne $expected) { throw 'Auto-close happened without a verified update.' }
        Write-Host 'PASS: successful helper exits without input even with -NoExit; caller remains running'
    } finally {
        if (-not $child.HasExited) { $child.Kill(); $child.WaitForExit() }
        $child.Dispose()
    }
} finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }
