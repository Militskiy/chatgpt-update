#requires -Version 5.1
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $version = (Get-Content VERSION -Raw).Trim()
    if ($version -notmatch '^\d+\.\d+\.\d+$') { throw 'Invalid VERSION.' }
    $hashes = [ordered]@{}
    foreach ($file in @(Get-ChildItem scripts -Filter *.ps1 | Sort-Object Name)) {
        $hashes[$file.Name] = (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    [IO.File]::WriteAllText((Join-Path $PSScriptRoot 'script-hashes.json'), ($hashes | ConvertTo-Json), (New-Object Text.UTF8Encoding($false)))
    $out = Join-Path $PSScriptRoot 'dist'
    if (Test-Path $out) { Remove-Item -LiteralPath $out -Recurse -Force }
    $portable = Join-Path $out 'portable'
    New-Item -ItemType Directory $portable -Force | Out-Null
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    & go build -trimpath -o (Join-Path $portable 'chatgpt-update.exe') .
    if ($LASTEXITCODE) { throw 'Go build failed.' }
    Copy-Item scripts $portable -Recurse
    Copy-Item VERSION,README.md $portable
    $names = @('chatgpt-update.exe','VERSION','README.md','scripts/path.ps1','scripts/prepare-state.ps1','scripts/replace-updater.ps1','scripts/update-chatgpt.ps1')
    $lines = foreach ($name in $names) {
        $hash = (Get-FileHash (Join-Path $portable $name) -Algorithm SHA256).Hash.ToLowerInvariant()
        '{0}  {1}' -f $hash, $name
    }
    [IO.File]::WriteAllText((Join-Path $portable 'FILES.sha256'), (($lines -join "`n") + "`n"), [Text.Encoding]::ASCII)
    # Windows PowerShell 5.1 Compress-Archive can emit backslashes in ZIP names.
    # Emit canonical forward-slash entries instead; do not weaken the strict
    # self-update ZIP allowlist to accept ambiguous separators or traversal.
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [IO.Compression.ZipFile]::Open((Join-Path $out 'chatgpt-update-windows-x64.zip'), [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($name in @($names) + @('FILES.sha256')) {
            [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $portable $name), $name, [IO.Compression.CompressionLevel]::Optimal)
        }
    } finally { $archive.Dispose() }
    $zipHash = (Get-FileHash (Join-Path $out 'chatgpt-update-windows-x64.zip') -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $out 'SHA256SUMS.txt'), "$zipHash  chatgpt-update-windows-x64.zip`n", [Text.Encoding]::ASCII)
    Write-Host "Built portable folder $version. Defender scan is REQUIRED before executing/publishing the built app."
}
finally { Pop-Location }
