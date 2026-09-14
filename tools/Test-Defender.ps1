#requires -Version 5.1
# Build-runner custom scan gate. Never changes antivirus preferences/exclusions,
# restores quarantine, disables security, or runs the files being scanned.
[CmdletBinding()]
param([string]$ScanPath=(Join-Path $PSScriptRoot '../dist'), [string]$ReportDirectory=(Join-Path $PSScriptRoot '../scan-results'))
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'; $global:LASTEXITCODE=0
New-Item -ItemType Directory $ReportDirectory -Force | Out-Null
$report=[ordered]@{ result='inconclusive'; startedUtc=[DateTime]::UtcNow.ToString('o'); scope='Custom scan of the complete portable folder and release ZIP before smoke tests'; policyChanges=$false; targetExecuted=$false; limitations='This is not a Microsoft analyst verdict, SmartScreen reputation check, or guarantee against endpoint/cloud detections.' }
try {
    $scanRoot=(Resolve-Path -LiteralPath $ScanPath).ProviderPath
    $before=@(Get-ChildItem -LiteralPath $scanRoot -Recurse -File | ForEach-Object { @{path=$_.FullName.Substring($scanRoot.Length+1);size=$_.Length;sha256=(Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()} })
    if ($before.Count -lt 8) { throw 'Expected distributable files were not built.' }
    $report['files']=$before
    $s=Get-MpComputerStatus
    if (-not $s.AMServiceEnabled -or -not $s.AntivirusEnabled) { throw 'Defender scan service is not active; release is blocked.' }
    $c=@(Get-ChildItem "$env:ProgramData\Microsoft\Windows Defender\Platform\*\MpCmdRun.exe" -ErrorAction SilentlyContinue | Sort-Object { $_.VersionInfo.FileVersionRaw } -Descending)
    $mp=if ($c.Count) {$c[0].FullName} else {"$env:ProgramFiles\Windows Defender\MpCmdRun.exe"}
    if (-not (Test-Path $mp)) { throw 'Defender scanner is unavailable.' }
    $report['scanner']=$mp
    & $mp -SignatureUpdate 2>&1 | Out-File (Join-Path $ReportDirectory 'signature-update.txt')
    $report['signatureUpdateExitCode']=$LASTEXITCODE
    if ($LASTEXITCODE) { throw 'Security intelligence update failed; no scan clearance is claimed.' }
    $s=Get-MpComputerStatus
    $report['defender']=$s | Select-Object AMServiceEnabled,AntivirusEnabled,RealTimeProtectionEnabled,BehaviorMonitorEnabled,IoavProtectionEnabled,AMProductVersion,AMEngineVersion,AntivirusSignatureVersion,AntivirusSignatureLastUpdated
    if ((Get-Date)-$s.AntivirusSignatureLastUpdated -gt [TimeSpan]::FromDays(2)) { throw 'Security intelligence is more than two days old.' }
    & $mp -ValidateMapsConnection 2>&1 | Out-File (Join-Path $ReportDirectory 'cloud-connection.txt')
    $report['cloudConnectionExitCode']=$LASTEXITCODE
    $report['cloudCoverageVerified']=($LASTEXITCODE -eq 0)
    # Scan-only mode ignores exclusions and includes archive contents. This
    # option prevents remediation ONLY for this scan, not real-time protection.
    & $mp -Scan -ScanType 3 -File $scanRoot -DisableRemediation 2>&1 | Out-File (Join-Path $ReportDirectory 'custom-scan.txt')
    $report['scanExitCode']=$LASTEXITCODE
    if ($LASTEXITCODE) { throw 'Defender reported a detection or scanning error; artifacts will not be published.' }
    # Exit 0 alone is not a sufficient integrity check: ensure no component
    # vanished or changed due to remediation/another process during scanning.
    foreach ($f in $before) {
        $p=Join-Path $scanRoot $f.path
        if (-not (Test-Path -LiteralPath $p) -or (Get-FileHash $p -Algorithm SHA256).Hash.ToLowerInvariant() -cne $f.sha256) { throw 'A scanned file changed/disappeared; release is blocked.' }
    }
    $scanText=Get-Content (Join-Path $ReportDirectory 'custom-scan.txt') -Raw
    # CI uses an English Windows image. Do not infer a pass from ambiguous output.
    if ($scanText -notmatch '(?i)found no threats') { throw 'Scanner did not explicitly report no threats; inspect raw output.' }
    $report['result']='custom-scan-no-detections'
    $report['stableReleaseApproved']=$false
    Write-Host 'PASS: updated-definition custom scan reported no threats; files unchanged.'
    if (-not $report.cloudCoverageVerified -or -not $s.RealTimeProtectionEnabled) { Write-Warning 'Cloud and/or real-time coverage is unavailable on this runner. This is NOT a clearance of the user-reported alert.' }
} catch {
    $report['error']=$_.Exception.Message
    throw
} finally {
    $report['finishedUtc']=[DateTime]::UtcNow.ToString('o')
    $report | ConvertTo-Json -Depth 8 | Set-Content (Join-Path $ReportDirectory 'defender-report.json') -Encoding UTF8
}
