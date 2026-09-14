#requires -Version 5.1
<#
Offline regression tests for v5.4 selection, prompts and catalog filtering.
Reads the updater and imports ONLY pure helper definitions using its AST.
Does not execute the updater's main body, contact any server or install anything.
No Pester dependency. Run under Windows PowerShell 5.1 or PowerShell 7.
#>
[CmdletBinding()]
param([string]$ScriptPath = (Join-Path $PSScriptRoot '../scripts/update-chatgpt.ps1'))
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$resolvedPath = (Get-Item -LiteralPath $ScriptPath).FullName
$parseTokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($resolvedPath, [ref]$parseTokens, [ref]$parseErrors)
if ($null -ne $parseErrors -and $parseErrors.Count -gt 0) {
    throw (($parseErrors | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine)
}
$functionNames = @('Get-ObjectPropertyValue', 'Assert-MirrorDownloadUrl', 'Get-MirrorCandidatesFromReleases', 'Select-TargetMirrorRelease', 'Get-InstallConfirmationPrompt', 'Get-TransferDisplay', 'ConvertTo-CurlArgument')
$definitions = @($ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $false))
foreach ($name in $functionNames) {
    $matchesForName = @($definitions | Where-Object { $_.Name -eq $name })
    if ($matchesForName.Count -ne 1) { throw ('Expected one definition for {0}.' -f $name) }
    . ([scriptblock]::Create($matchesForName[0].Extent.Text))
}
$MirrorRepo = 'Wangnov/codex-app-mirror'
$script:Passed = 0
function Assert-Equal($Actual, $Expected, [string]$Name) {
    if ([string]$Actual -cne [string]$Expected) {
        throw ('FAILED: {0}. Expected <{1}>, received <{2}>.' -f $Name, $Expected, $Actual)
    }
    $script:Passed++
    Write-Host ('PASS: {0}' -f $Name)
}
$selectionCases = @(
    @{ Name = 'reported mirror ahead'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.903.9818.0'); Exact = $false; Expected = '26.908.4834.0' }
    @{ Name = 'reported exact only'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.903.9818.0'); Exact = $true; Expected = $null }
    @{ Name = 'newest ahead even exact exists'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4561.0', '26.908.4834.0'); Exact = $false; Expected = '26.908.4834.0' }
    @{ Name = 'exact exists under higher'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.908.4561.0', '26.903.9818.0'); Exact = $true; Expected = '26.908.4561.0' }
    @{ Name = 'installed equals feed, higher mirror'; Installed = '26.908.4561.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0'); Exact = $false; Expected = '26.908.4834.0' }
    @{ Name = 'installed ahead of feed, newer mirror'; Installed = '26.908.4834.0'; Feed = '26.908.4561.0'; Versions = @('26.908.5000.0', '26.908.4834.0'); Exact = $false; Expected = '26.908.5000.0' }
    @{ Name = 'installed ahead, same mirror'; Installed = '26.908.4834.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.903.9818.0'); Exact = $false; Expected = $null }
    @{ Name = 'installed ahead, older mirror'; Installed = '26.908.4834.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4561.0', '26.903.9818.0'); Exact = $false; Expected = $null }
    @{ Name = 'exact would downgrade'; Installed = '26.908.4834.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4561.0'); Exact = $true; Expected = $null }
    @{ Name = 'exact would reinstall'; Installed = '26.908.4561.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4561.0', '26.908.4834.0'); Exact = $true; Expected = $null }
    @{ Name = 'default exact when highest'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4561.0', '26.903.9818.0'); Exact = $false; Expected = '26.908.4561.0' }
    @{ Name = 'mirror behind but upgrade'; Installed = '26.903.8094.0'; Feed = '26.908.4561.0'; Versions = @('26.903.9818.0', '26.903.8094.0'); Exact = $false; Expected = '26.903.9818.0' }
    @{ Name = 'mirror behind exact only'; Installed = '26.903.8094.0'; Feed = '26.908.4561.0'; Versions = @('26.903.9818.0', '26.903.8094.0'); Exact = $true; Expected = $null }
    @{ Name = 'behind mirror equals installed'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.903.9818.0'); Exact = $false; Expected = $null }
    @{ Name = 'fresh install ahead'; Installed = $null; Feed = '26.908.4561.0'; Versions = @('26.903.9818.0', '26.908.4834.0'); Exact = $false; Expected = '26.908.4834.0' }
    @{ Name = 'fresh install behind'; Installed = $null; Feed = '26.908.4561.0'; Versions = @('26.903.9818.0'); Exact = $false; Expected = '26.903.9818.0' }
    @{ Name = 'fresh install exact'; Installed = $null; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.908.4561.0'); Exact = $true; Expected = '26.908.4561.0' }
    @{ Name = 'fresh install exact missing'; Installed = $null; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.903.9818.0'); Exact = $true; Expected = $null }
    @{ Name = 'empty upgrade'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @(); Exact = $false; Expected = $null }
    @{ Name = 'empty fresh'; Installed = $null; Feed = '26.908.4561.0'; Versions = @(); Exact = $false; Expected = $null }
    @{ Name = 'numeric sort'; Installed = $null; Feed = '26.908.9000.0'; Versions = @('26.908.9818.0', '26.908.10000.0'); Exact = $false; Expected = '26.908.10000.0' }
    @{ Name = 'unsorted'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.5000.0', '26.903.9818.0', '26.908.4834.0', '26.908.4561.0'); Exact = $false; Expected = '26.908.5000.0' }
    @{ Name = 'duplicate versions'; Installed = '26.903.9818.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.908.4834.0', '26.908.4561.0'); Exact = $false; Expected = '26.908.4834.0' }
    @{ Name = 'all older than installed'; Installed = '26.908.5000.0'; Feed = '26.908.4561.0'; Versions = @('26.908.4834.0', '26.903.9818.0', '26.908.4561.0'); Exact = $false; Expected = $null }
)
foreach ($case in $selectionCases) {
    $inputCandidates = @($case.Versions | ForEach-Object { [pscustomobject]@{ Version = [version]$_ } })
    $result = Select-TargetMirrorRelease -AdvertisedVersion ([version]$case.Feed) -InstalledVersion $case.Installed -Candidates $inputCandidates -ExactOnly:$case.Exact
    $actualVersion = $null
    if ($null -ne $result) { $actualVersion = [string]$result.Version }
    Assert-Equal $actualVersion $case.Expected $case.Name
}
Assert-Equal (Get-InstallConfirmationPrompt -TargetVersion '26.908.4834.0' -AdvertisedVersion '26.908.4561.0' -IsUpgrade) 'Install mirrored 26.908.4834.0, NEWER than OpenAI feed 26.908.4561.0?' 'ahead-of-feed upgrade prompt'
Assert-Equal (Get-InstallConfirmationPrompt -TargetVersion '26.908.4834.0' -AdvertisedVersion '26.908.4561.0') 'Install mirrored 26.908.4834.0, NEWER than OpenAI feed 26.908.4561.0?' 'ahead-of-feed fresh prompt'
Assert-Equal (Get-InstallConfirmationPrompt -TargetVersion '26.903.9818.0' -AdvertisedVersion '26.908.4561.0' -IsUpgrade) 'Install available 26.903.9818.0 instead of advertised 26.908.4561.0?' 'older available upgrade prompt'
Assert-Equal (Get-InstallConfirmationPrompt -TargetVersion '26.903.9818.0' -AdvertisedVersion '26.908.4561.0') 'Install available 26.903.9818.0 instead of advertised 26.908.4561.0?' 'older available fresh prompt'
Assert-Equal (Get-InstallConfirmationPrompt -TargetVersion '26.908.4561.0' -AdvertisedVersion '26.908.4561.0' -IsUpgrade) 'Install update 26.908.4561.0?' 'exact upgrade prompt'
Assert-Equal (Get-InstallConfirmationPrompt -TargetVersion '26.908.4561.0' -AdvertisedVersion '26.908.4561.0') 'Install ChatGPT/Codex 26.908.4561.0 for this Windows user?' 'exact fresh prompt'

function New-TestAsset {
    param([string]$Name, [string]$State = 'uploaded', [string]$BaseUrl = 'https://github.com/Wangnov/codex-app-mirror/releases/download/test/')
    return [pscustomobject]@{ name = $Name; state = $State; size = [int64]800000000; browser_download_url = $BaseUrl + $Name }
}
function New-TestRelease {
    param([object[]]$Assets, [bool]$Draft = $false, [bool]$Prerelease = $false)
    return [pscustomobject]@{ name = 'Test release - tag is not a package version'; tag_name = '99.999.99999'; draft = $Draft; prerelease = $Prerelease; assets = $Assets }
}
$stableAsset = New-TestAsset 'OpenAI.Codex_26.908.4834.0_x64__2p2nqsd0c76g0.Msix'
$checksums = New-TestAsset 'SHA256SUMS.txt'
$filtered = @(Get-MirrorCandidatesFromReleases -Releases @((New-TestRelease -Assets @(
    $stableAsset,
    $checksums,
    (New-TestAsset 'OpenAI.CodexBeta_26.908.9999.0_x64__2p2nqsd0c76g0.Msix'),
    (New-TestAsset 'OpenAI.Codex_26.908.9999.0_arm64__2p2nqsd0c76g0.Msix'),
    (New-TestAsset 'OpenAI.Codex_26.908.9999.0_x64__wrongpublisher.Msix'),
    (New-TestAsset 'OpenAI.Codex_26.908.65536.0_x64__2p2nqsd0c76g0.Msix'),
    (New-TestAsset 'OpenAI.Codex_26.908.9999.0_x64__2p2nqsd0c76g0.Msix' -State 'new'),
    (New-TestAsset 'Codex-mac-x64.dmg')
))))
Assert-Equal $filtered.Count 1 'exclude Beta/ARM64/wrong family/invalid version/unuploaded/macOS assets'
Assert-Equal $filtered[0].Version '26.908.4834.0' 'compare Windows MSIX version rather than tag'
Assert-Equal $filtered[0].ChecksumUrl $checksums.browser_download_url 'retain checksum asset'
Assert-Equal @(Get-MirrorCandidatesFromReleases -Releases @((New-TestRelease -Assets @($stableAsset) -Draft $true))).Count 0 'exclude draft releases'
Assert-Equal @(Get-MirrorCandidatesFromReleases -Releases @((New-TestRelease -Assets @($stableAsset) -Prerelease $true))).Count 0 'exclude prereleases'
Assert-Equal @(Get-MirrorCandidatesFromReleases -Releases @()).Count 0 'empty catalog'
$rejected = $false
try {
    $null = @(Get-MirrorCandidatesFromReleases -Releases @((New-TestRelease -Assets @(
        (New-TestAsset 'OpenAI.Codex_26.908.4834.0_x64__2p2nqsd0c76g0.Msix' -BaseUrl 'https://example.com/download/')
    ))))
}
catch { $rejected = $true }
Assert-Equal $rejected $true 'reject asset outside the configured release repository'
$source = Get-Content -LiteralPath $resolvedPath -Raw
Assert-Equal ([regex]::IsMatch($source, '\$[A-Za-z_]\w*\?')) $false 'no unbraced variable/question-mark interpolation'
Assert-Equal ($source.IndexOf('if ($CheckOnly)') -lt $source.IndexOf("`$stage = 'Confirming installation'")) $true 'check-only precedes installation consent'

# UI value formatting and native argument quoting are tested without starting curl.
$d = Get-TransferDisplay -Received (50MB) -Expected (100MB) -BytesPerSecond (5MB)
Assert-Equal $d.Percent 50 'download percentage is derived from bytes'
Assert-Equal ([regex]::Matches($d.Bar, '%').Count) 1 'one percentage in the progress bar'
Assert-Equal ([regex]::Matches($d.Bar, '#').Count) 14 'half of the bar is filled at 50 percent'
Assert-Equal $d.Status.EndsWith('ETA 00:10') $true 'ETA is calculated from bytes remaining and measured speed'
$d = Get-TransferDisplay -Received (100MB) -Expected (100MB) -BytesPerSecond (5MB)
Assert-Equal $d.Percent 99.9 'never announce 100 percent before download has exited'
$d = Get-TransferDisplay -Received (100MB) -Expected (100MB) -BytesPerSecond (5MB) -Completed
Assert-Equal $d.Percent 100 '100 percent on completed matching-size download'
$d = Get-TransferDisplay -Received (-1) -Expected (100MB) -BytesPerSecond 0
Assert-Equal $d.Percent 0 'clamp negative byte input'
Assert-Equal $d.Status.EndsWith('ETA --:--') $true 'no invented ETA while waiting'
$d = Get-TransferDisplay -Received (1MB) -Expected 0 -BytesPerSecond 0
Assert-Equal $d.Percent (-1) 'unknown length is indeterminate'
Assert-Equal $d.Bar.Contains('%') $false 'no fake percent for unknown length'
Assert-Equal (ConvertTo-CurlArgument 'C:\A B\update.msix') '"C:\A B\update.msix"' 'quote Windows paths containing spaces'
Assert-Equal (ConvertTo-CurlArgument 'https://example.test/a?x=1&y=2') '"https://example.test/a?x=1&y=2"' 'URL passes as a single native argument'
$rejected = $false
try { $null = ConvertTo-CurlArgument 'bad"argument' } catch { $rejected = $true }
Assert-Equal $rejected $true 'reject a double quote in a native argument'
Assert-Equal ($source.Contains("'--silent', '--show-error'")) $true 'curl transfer table is suppressed while errors remain'
Assert-Equal ($source.IndexOf('if ($Preview)') -lt $source.IndexOf("    Initialize-AppxCommands")) $true 'preview occurs before loading AppX commands'

Write-Host ('All {0} offline tests passed. No network/download/install operations were run.' -f $script:Passed) -ForegroundColor Green
