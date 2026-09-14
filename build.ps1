#requires -Version 5.1
[CmdletBinding()]
param()
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Install a supported Go SDK to build; running the app does not need Go.' }
    $version = (Get-Content VERSION -Raw).Trim()
    if ($version -notmatch '^\d+\.\d+\.\d+$') { throw 'VERSION must contain a stable three-part version.' }
    New-Item -ItemType Directory -Path dist -Force | Out-Null
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    & go build -trimpath -ldflags '-s -w' -o dist/chatgpt-update.exe .
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
    $sha = (Get-FileHash dist/chatgpt-update.exe -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $PSScriptRoot 'dist/SHA256SUMS.txt'), "$sha  chatgpt-update.exe`n", [Text.Encoding]::ASCII)
    Write-Host ('Built portable chatgpt-update.exe {0} (Windows x64).' -f $version)
}
finally { Pop-Location }
