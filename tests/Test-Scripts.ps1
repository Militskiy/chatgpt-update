#requires -Version 5.1
# Offline syntax/smoke tests; never installs ChatGPT or changes real PATH/state.
[CmdletBinding()]
param([string]$Executable = (Join-Path $PSScriptRoot '../dist/chatgpt-update.exe'))
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$global:LASTEXITCODE = 0
$root = Split-Path $PSScriptRoot -Parent
foreach ($file in @(Get-ChildItem (Join-Path $root 'scripts') -Filter *.ps1)) {
    $tokens = $null; $errors = $null
    $null = [Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) { throw ($errors | Out-String) }
    Write-Host ('PASS: parse {0}' -f $file.Name)
}
& (Join-Path $PSScriptRoot 'Test-UpdateEngine.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Engine tests failed.' }
$expected = (Get-Content (Join-Path $root 'VERSION') -Raw).Trim()
$reported = (& $Executable --version | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $reported -cne $expected) { throw 'EXE version smoke test failed.' }
& $Executable --help
if ($LASTEXITCODE -ne 0) { throw 'EXE help smoke test failed.' }
# Exercise the replacement helper against copies in a temporary directory only.
$temp = Join-Path $env:TEMP ('ChatGPTUpdater-selftest-' + [Guid]::NewGuid().ToString('N'))
try {
    New-Item -ItemType Directory -Path $temp | Out-Null
    $target = Join-Path $temp 'chatgpt-update.exe'
    Copy-Item -LiteralPath $Executable -Destination $target
    $stage = Join-Path $temp '.chatgpt-update-stage-test'
    New-Item -ItemType Directory -Path $stage | Out-Null
    $candidate = Join-Path $stage 'chatgpt-update.exe'
    Copy-Item -LiteralPath $Executable -Destination $candidate
    $plan = @{ parentPid = 2147483000; target = $target; candidate = $candidate
        sha256 = (Get-FileHash $candidate -Algorithm SHA256).Hash.ToLowerInvariant()
        newVersion = $expected; previous = $target + '.test.previous'; log = (Join-Path $temp 'replace.log') }
    $planPath = Join-Path $stage 'replacement.json'
    $plan | ConvertTo-Json | Set-Content -LiteralPath $planPath -Encoding UTF8
    & (Join-Path $root 'scripts/replace-updater.ps1') -PlanPath $planPath -NoPause
    if ($LASTEXITCODE -ne 0) { throw 'Replacement helper smoke test failed.' }
    if (-not (Test-Path -LiteralPath $plan.previous)) { throw 'Previous EXE was not preserved.' }
    if ((& $target --version | Out-String).Trim() -cne $expected) { throw 'Replaced EXE version is wrong.' }
    Write-Host 'PASS: self-update replacement and rollback-copy smoke test (temporary files only)'
}
finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }
