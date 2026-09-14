#requires -Version 5.1
# Load only presentation functions; do not execute any network or deployment.
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$file=Join-Path $PSScriptRoot '../scripts/update-chatgpt.ps1'
$tokens=$null;$errors=$null
$ast=[Management.Automation.Language.Parser]::ParseFile((Resolve-Path $file),[ref]$tokens,[ref]$errors)
if ($errors.Count) { throw ($errors|Out-String) }
foreach($name in @('Get-UiMarker','Get-UiStateColor','Get-UiLines')) {
    $f=@($ast.FindAll({param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst]},$true)|Where-Object Name -eq $name)
    if($f.Count -ne 1){throw "Missing UI function $name"}
    . ([scriptblock]::Create($f[0].Extent.Text))
}
if((Get-UiMarker Running -Animate -Frame 0) -ceq (Get-UiMarker Running -Animate -Frame 1)){throw 'Spinner does not move'}
if((Get-UiMarker Running -Frame 3) -cne '[>>]  '){throw 'Reduced motion marker changed'}
if((Get-UiStateColor Failed) -ne [ConsoleColor]::Red){throw 'Failure color'}
if((Get-UiStateColor Done) -ne [ConsoleColor]::Green){throw 'Completion color'}
$UpdaterVersion='test'
$script:Ui=@{Steps=@([pscustomobject]@{State='Running';Label='Current task'}); Motion=$false;Live=$false;StepStarted=[DateTime]::UtcNow.AddSeconds(-4);Detail='Test';TransferBar='';Transfer=''}
$lines=@(Get-UiLines -Width 35)
if(@($lines|Where-Object Length -gt 35).Count){throw 'UI lines wrap'}
if(($lines -join "`n") -notmatch '\[>>\]'){throw 'Static marker missing'}
Write-Host 'PASS: PowerShell UI markers, colors, elapsed label and width limits'
