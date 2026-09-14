#requires -Version 5.1
[CmdletBinding()]
param([ValidateSet('add','remove')] [string]$Action, [Parameter(Mandatory=$true)] [string]$Directory)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
try {
    $dir = [IO.Path]::GetFullPath($Directory).TrimEnd('\')
    if ($dir -match '[;\r\n]') { throw 'PATH cannot represent a folder with semicolons or newlines.' }
    if (-not (Test-Path -LiteralPath (Join-Path $dir 'chatgpt-update.exe') -PathType Leaf)) { throw 'Keep the executable named chatgpt-update.exe in this folder.' }
    $old = [Environment]::GetEnvironmentVariable('Path', [EnvironmentVariableTarget]::User)
    $kept = New-Object 'System.Collections.Generic.List[string]'
    $found = $false
    foreach ($entry in @(([string]$old) -split ';')) {
        if ([string]::IsNullOrWhiteSpace($entry)) { continue }
        $candidate = [Environment]::ExpandEnvironmentVariables($entry.Trim().Trim('"')).TrimEnd('\')
        if ($candidate -ieq $dir) { $found = $true; if ($Action -eq 'add') { $kept.Add($entry) } }
        else { $kept.Add($entry) }
    }
    if ($Action -eq 'add' -and -not $found) { $kept.Add($dir) }
    $new = $kept -join ';'
    if ($new -cne [string]$old) {
        [Environment]::SetEnvironmentVariable('Path', $new, [EnvironmentVariableTarget]::User)
        Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class PathChangeNotification {
    [DllImport("user32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    public static extern IntPtr SendMessageTimeout(IntPtr h, uint msg, UIntPtr w, string l, uint flags, uint timeout, out UIntPtr result);
}
'@
        $result = [UIntPtr]::Zero
        [void][PathChangeNotification]::SendMessageTimeout([IntPtr]0xffff, 0x001a, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result)
        Write-Host '[OK] Updated USER PATH only. Machine PATH was not changed.'
    } else { Write-Host '[OK] No PATH change was needed.' }
    Write-Host 'Close all terminal windows and open a NEW terminal from Start before using chatgpt-update by name.'
    Write-Host ('Portable folder: {0}' -f $dir)
}
catch { Write-Host $_.Exception.Message -ForegroundColor Red; exit 1 }
