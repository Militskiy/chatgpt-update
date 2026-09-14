#requires -Version 5.1
<#
.SYNOPSIS
Installs or updates the stable ChatGPT/Codex Windows desktop MSIX for this user.
.DESCRIPTION
Version 5.4.0. Intended for Windows x64 with PowerShell 5.1 or PowerShell 7.
Checks OpenAI's buildVersion feed, then asks Y/N before installation or updating.
On upgrades, asks separately whether to back up the user's .codex directory.
Fresh installations do not create a backup. Never uninstalls an existing app.

IMPORTANT: The MSIX is downloaded from the THIRD-PARTY GitHub mirror
Wangnov/codex-app-mirror, not directly from OpenAI. This preserves the source
used in v4. By default, this script OFFERS the highest compatible Windows package
version in the scanned, non-prerelease mirror releases. OpenAI's advertised build
is a reference, not an upper version limit. A build above the feed is explicitly
labelled as NOT confirmed by that feed; higher version does not prove stability
or authenticity. All upgrades must be strictly newer than the installed app.
The exact target is shown before Y/N consent; no version is substituted silently.
Use -ExactVersionOnly to require an exact advertised-version match instead.
The package family is derived from the manifest publisher; Windows performs
signature/trust validation during Add-AppxPackage. A mirror-provided checksum
alone is NOT proof of publisher authenticity.

Only use this method where your organization's policies permit it. The script
does not change Store policy, enable Developer Mode, import certificates,
disable signature checks, or install dependencies from guessed download URLs.

Backup scope: $HOME\.codex only (settings/history/state, where present). This is
not a backup of all desktop app data, project repositories or other locations.
Backups can contain credentials; keep them private.

Sources consulted for this edit:
https://learn.microsoft.com/powershell/module/appx/add-appxpackage
https://learn.microsoft.com/windows/win32/api/appmodel/nf-appmodel-packagefamilynamefromid
https://github.com/Wangnov/codex-app-mirror
.PARAMETER NoBackup
Skip the backup and its prompt when upgrading. Installation still requires Y/N.
.PARAMETER Backup
Create the .codex backup without asking a separate backup question on upgrades.
Installation still requires Y/N. Do not combine with -NoBackup.
.PARAMETER ExactVersionOnly
Only offer the exact OpenAI-advertised build, never an older or newer alternative.
The exact build is selected even if the mirror also contains higher versions.
If it is absent or not newer than the installed app, exit without app changes.
This option never permits downgrades/reinstalls or skips the Y/N prompt.
.PARAMETER CheckOnly
Check installed, advertised and mirrored versions without prompts, installer
files, backups, closing the app or installing anything.
.PARAMETER Preview
Run a short, simulated step/progress demonstration. No network or installation,
no AppX commands, no files, no app closure, and no backup. Works without admin.
.PARAMETER PlainOutput
Use ordinary scrolling text instead of a live in-place step checklist. The
same download/installation checks and Y/N prompts still apply. Automatically
used when a suitable interactive console is not available.
.PARAMETER DependencyPath
Optional local, signed dependency .appx/.msix packages or bundles obtained from
an approved source. Passed to Add-AppxPackage together with the main package.
No dependency files are downloaded or installed automatically by this script.
.EXAMPLE
.\Install-Update-ChatGPT-Corp-PS5-PS7-v5.4.ps1
.EXAMPLE
.\Install-Update-ChatGPT-Corp-PS5-PS7-v5.4.ps1 -NoBackup
.EXAMPLE
.\Install-Update-ChatGPT-Corp-PS5-PS7-v5.4.ps1 -Backup
#>

[CmdletBinding()]
param(
    [switch]$NoBackup,
    [switch]$Backup,
    [switch]$ExactVersionOnly,
    [switch]$CheckOnly,
    [switch]$Preview,
    [switch]$PlainOutput,
    [string[]]$DependencyPath = @()
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$UpdaterVersion = '5.4.0'
$PackageName = 'OpenAI.Codex'
$ExpectedPackageFamily = 'OpenAI.Codex_2p2nqsd0c76g0'
$OpenAIManifestUrl = 'https://persistent.oaistatic.com/codex-app-prod/windows-store-update.json'
$MirrorApi = 'https://api.github.com/repos/Wangnov/codex-app-mirror/releases'
$MirrorRepo = 'Wangnov/codex-app-mirror'

# Do not suppress AppX deployment errors. Useful state for the error message.
$workDir = $null
$backupPath = $null
$stage = 'Startup'


# UI only: ASCII markers work in Windows PowerShell 5.1 and PowerShell 7.
# No ANSI terminal support, modules, fonts or GUI libraries are required.
$script:Ui = @{
    Enabled = $false; Live = $false; Visible = $false; Top = 0; Width = 0; Height = 0
    Current = 0; Detail = ''; Transfer = ''; TransferBar = ''; LastBucket = -1
    Steps = @(
        [pscustomobject]@{ Label = 'Check installed app'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Find an available update'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Confirm installation and backup'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Download installer / reuse cache'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Validate package and prerequisites'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Close the existing app'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Optional .codex backup'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Install / update the app'; State = 'Pending' }
        [pscustomobject]@{ Label = 'Verify installed version'; State = 'Pending' }
    )
}

function Get-UiLines {
    param([int]$Width = 78)
    $limit = [Math]::Max(30, $Width)
    $lines = New-Object 'System.Collections.Generic.List[string]'
    $lines.Add(('ChatGPT/Codex setup v{0}' -f $UpdaterVersion))
    $lines.Add(('-' * [Math]::Min(70, $limit)))
    for ($i = 0; $i -lt $script:Ui.Steps.Count; $i++) {
        $marker = switch ($script:Ui.Steps[$i].State) {
            'Done'    { '[OK]  ' }
            'Running' { '[>>]  ' }
            'Skipped' { '[SKIP]' }
            'Failed'  { '[FAIL]' }
            default   { '[ .. ]' }
        }
        $lines.Add(('{0} {1}. {2}' -f $marker, ($i + 1), $script:Ui.Steps[$i].Label))
    }
    $lines.Add('')
    $lines.Add(('  {0}' -f $script:Ui.Detail))
    $lines.Add(('  {0}' -f $script:Ui.TransferBar))
    $lines.Add(('  {0}' -f $script:Ui.Transfer))
    foreach ($line in $lines) {
        # Never wrap the reserved panel: wrapping would change cursor positions.
        $clean = [regex]::Replace($line, '[\x00-\x1f\x7f]', ' ')
        if ($clean.Length -gt $limit) { $clean = $clean.Substring(0, $limit - 3) + '...' }
        Write-Output $clean
    }
}

function Hide-UiPanel {
    if (-not $script:Ui.Visible) { return }
    try {
        $width = [Math]::Min($script:Ui.Width, [Console]::BufferWidth - 1)
        if ($width -lt 1 -or $script:Ui.Top -lt 0 -or
            ($script:Ui.Top + $script:Ui.Height) -ge [Console]::BufferHeight) {
            throw 'Console size changed.'
        }
        for ($i = 0; $i -lt $script:Ui.Height; $i++) {
            [Console]::SetCursorPosition(0, $script:Ui.Top + $i)
            [Console]::Write((' ' * $width))
        }
        [Console]::SetCursorPosition(0, $script:Ui.Top)
    }
    catch {
        # A display failure must never stop an install. Fall back to plain output.
        $script:Ui.Live = $false
    }
    $script:Ui.Visible = $false
}

function Show-UiPanel {
    if (-not $script:Ui.Enabled -or -not $script:Ui.Live) { return }
    $oldColor = $null
    try {
        $width = [Math]::Min(94, [Math]::Min([Console]::WindowWidth, [Console]::BufferWidth) - 2)
        $lines = @(Get-UiLines -Width $width)
        if ($width -lt 58 -or [Console]::WindowHeight -lt ($lines.Count + 3)) {
            Hide-UiPanel
            $script:Ui.Live = $false
            return
        }
        if ($script:Ui.Visible -and $script:Ui.Width -ne $width) { Hide-UiPanel }
        if (-not $script:Ui.Live) { return }
        $oldColor = [Console]::ForegroundColor
        if (-not $script:Ui.Visible) {
            # Reserve rows first; CursorTop reflects any scrolling at the bottom.
            if ([Console]::CursorLeft -ne 0) { [Console]::WriteLine() }
            foreach ($line in $lines) { [Console]::WriteLine() }
            $script:Ui.Top = [Console]::CursorTop - $lines.Count
            $script:Ui.Height = $lines.Count
            $script:Ui.Width = $width
            $script:Ui.Visible = $true
        }
        for ($i = 0; $i -lt $lines.Count; $i++) {
            [Console]::SetCursorPosition(0, $script:Ui.Top + $i)
            [Console]::ForegroundColor = $oldColor
            if ($lines[$i].StartsWith('[OK]')) { [Console]::ForegroundColor = [ConsoleColor]::Green }
            elseif ($lines[$i].StartsWith('[>>]')) { [Console]::ForegroundColor = [ConsoleColor]::Cyan }
            elseif ($lines[$i].StartsWith('[SKIP]')) { [Console]::ForegroundColor = [ConsoleColor]::DarkGray }
            elseif ($lines[$i].StartsWith('[FAIL]')) { [Console]::ForegroundColor = [ConsoleColor]::Red }
            [Console]::Write($lines[$i].PadRight($width))
        }
        [Console]::SetCursorPosition(0, $script:Ui.Top + $script:Ui.Height)
    }
    catch {
        $script:Ui.Visible = $false
        $script:Ui.Live = $false
    }
    finally {
        if ($null -ne $oldColor) { try { [Console]::ForegroundColor = $oldColor } catch { } }
    }
}

function Write-UiMessage {
    param(
        [Parameter(Position = 0)] [AllowNull()] [AllowEmptyString()] [object]$Object = '',
        [ConsoleColor]$ForegroundColor,
        [switch]$NoNewline
    )
    Hide-UiPanel
    $arguments = @{ Object = $Object; NoNewline = $NoNewline }
    if ($PSBoundParameters.ContainsKey('ForegroundColor')) {
        $arguments['ForegroundColor'] = $ForegroundColor
    }
    Microsoft.PowerShell.Utility\Write-Host @arguments
    if (-not $NoNewline) { Show-UiPanel }
}

function Initialize-Ui {
    $script:Ui.Enabled = $true
    $script:Ui.Live = $false
    try {
        $script:Ui.Live = (-not $PlainOutput -and $Host.Name -eq 'ConsoleHost' -and
            -not [Console]::IsOutputRedirected -and -not [Console]::IsInputRedirected -and
            [Console]::WindowWidth -ge 60 -and [Console]::WindowHeight -ge 20)
    }
    catch { $script:Ui.Live = $false }
    if (-not $script:Ui.Live) {
        Microsoft.PowerShell.Utility\Write-Host 'Steps: check app -> find update -> confirm -> download -> validate -> close -> backup -> install -> verify'
    }
}

function Set-UiStep {
    param([ValidateRange(1, 9)] [int]$Number, [string]$Detail = '')
    $previous = $script:Ui.Current
    if ($previous -ne $Number -and $previous -gt 0 -and
        $script:Ui.Steps[$previous - 1].State -eq 'Running') {
        $script:Ui.Steps[$previous - 1].State = 'Done'
        if (-not $script:Ui.Live) {
            Write-UiMessage ('[OK] {0}/9 {1}' -f $previous, $script:Ui.Steps[$previous - 1].Label)
        }
    }
    $script:Ui.Current = $Number
    $script:Ui.Steps[$Number - 1].State = 'Running'
    $script:Ui.Detail = $Detail
    if ($previous -ne $Number) {
        $script:Ui.Transfer = ''
        $script:Ui.TransferBar = ''
        $script:Ui.LastBucket = -1
        if (-not $script:Ui.Live) {
            Write-UiMessage ('[>>] {0}/9 {1}' -f $Number, $script:Ui.Steps[$Number - 1].Label) -ForegroundColor Cyan
        }
    }
    Show-UiPanel
}

function Skip-UiStep {
    param([ValidateRange(1, 9)] [int]$Number)
    $script:Ui.Steps[$Number - 1].State = 'Skipped'
    if (-not $script:Ui.Live) {
        Write-UiMessage ('[SKIP] {0}/9 {1}' -f $Number, $script:Ui.Steps[$Number - 1].Label)
    }
    Show-UiPanel
}

function Finish-UiSteps {
    param([string]$Message = 'Finished.', [switch]$Failed)
    $current = $script:Ui.Current
    if ($current -gt 0 -and $script:Ui.Steps[$current - 1].State -eq 'Running') {
        if ($Failed) { $script:Ui.Steps[$current - 1].State = 'Failed' }
        else { $script:Ui.Steps[$current - 1].State = 'Done' }
    }
    if (-not $Failed) {
        foreach ($step in $script:Ui.Steps) {
            if ($step.State -eq 'Pending') { $step.State = 'Skipped' }
        }
    }
    $script:Ui.Detail = $Message
    $script:Ui.TransferBar = ''
    $script:Ui.Transfer = ''
    Show-UiPanel
}

function Close-Ui {
    if (-not $script:Ui.Enabled) { return }
    if ($script:Ui.Current -gt 0 -and $script:Ui.Steps[$script:Ui.Current - 1].State -eq 'Running') {
        # Includes Ctrl+C; do not report a completed install when interrupted.
        $script:Ui.Detail = 'Stopped before completion.'
    }
    Hide-UiPanel
    $script:Ui.Enabled = $false
    foreach ($line in @(Get-UiLines)) { Microsoft.PowerShell.Utility\Write-Host $line }
    Microsoft.PowerShell.Utility\Write-Host ''
}

function Get-TransferDisplay {
    param([long]$Received, [long]$Expected, [double]$BytesPerSecond, [switch]$Completed)
    $receivedSafe = [Math]::Max(0, $Received)
    $percentage = -1.0
    $bar = 'Download in progress ...'
    $size = ('{0:N1} MiB received' -f ($receivedSafe / 1MB))
    if ($Expected -gt 0) {
        $percentage = [Math]::Max(0.0, [Math]::Min(99.9, 100.0 * $receivedSafe / $Expected))
        if ($Completed) { $percentage = 100.0 }
        $filled = [int][Math]::Floor(28.0 * $percentage / 100.0)
        $bar = '[{0}{1}] {2,5:N1}%' -f ('#' * $filled), ('-' * (28 - $filled)), $percentage
        $size = '{0:N1} / {1:N1} MiB' -f ($receivedSafe / 1MB), ($Expected / 1MB)
    }
    $rate = 'connecting / waiting'
    $eta = '--:--'
    if ($BytesPerSecond -gt 0) {
        $rate = '{0:N1} MiB/s' -f ($BytesPerSecond / 1MB)
        if ($Expected -gt 0) {
            $seconds = [long][Math]::Ceiling([Math]::Max(0, $Expected - $receivedSafe) / $BytesPerSecond)
            if ($seconds -le 359999) {
                $duration = [TimeSpan]::FromSeconds($seconds)
                if ($seconds -ge 3600) { $eta = '{0:00}:{1:00}:{2:00}' -f [int][Math]::Floor($duration.TotalHours), $duration.Minutes, $duration.Seconds }
                else { $eta = '{0:00}:{1:00}' -f $duration.Minutes, $duration.Seconds }
            }
        }
    }
    if ($Completed) { $rate = 'download complete'; $eta = '00:00' }
    [pscustomobject]@{
        Bar = $bar
        Status = ('{0} | {1} | ETA {2}' -f $size, $rate, $eta)
        Percent = $percentage
    }
}

function Update-UiTransfer {
    param([long]$Received, [long]$Expected, [double]$BytesPerSecond, [switch]$Completed)
    $display = Get-TransferDisplay -Received $Received -Expected $Expected -BytesPerSecond $BytesPerSecond -Completed:$Completed
    $script:Ui.TransferBar = $display.Bar
    $script:Ui.Transfer = $display.Status
    if ($script:Ui.Live) { Show-UiPanel }
    else {
        # Log/plain mode: one progress line per 10%, not hundreds of timed rows.
        $bucket = [int][Math]::Floor([Math]::Max(0, $display.Percent) / 10)
        if ($Completed -or $bucket -gt $script:Ui.LastBucket) {
            Write-UiMessage ('  {0}  {1}' -f $display.Bar, $display.Status)
            $script:Ui.LastBucket = $bucket
        }
    }
}

function Invoke-UiPreview {
    Write-UiMessage 'PREVIEW ONLY: simulated data. No network, downloads, backups, app closure or installation.' -ForegroundColor Yellow
    Set-UiStep 1 'Detecting the installed app (demo)'
    Start-Sleep -Milliseconds 500
    Set-UiStep 2 'Finding the available version (demo)'
    Start-Sleep -Milliseconds 500
    Set-UiStep 3 'Y/N confirmation (simulated approval; no real install)'
    Start-Sleep -Milliseconds 500
    Skip-UiStep 7
    Set-UiStep 4 'Downloading the MSIX (SIMULATED)'
    $demoSize = [long](739 * 1MB)
    for ($i = 0; $i -le 100; $i += 5) {
        Update-UiTransfer -Received ([long]($demoSize * $i / 100)) -Expected $demoSize -BytesPerSecond (12 * 1MB) -Completed:($i -eq 100)
        Start-Sleep -Milliseconds 160
    }
    Set-UiStep 5 'Checking checksum, identity and dependencies (demo)'
    Start-Sleep -Milliseconds 700
    Set-UiStep 6 'Closing the app (demo only)'
    Start-Sleep -Milliseconds 500
    Set-UiStep 8 'Installing... percentage is not estimated (demo)'
    Start-Sleep -Milliseconds 1200
    Set-UiStep 9 'Verifying version (demo)'
    Start-Sleep -Milliseconds 500
    Finish-UiSteps 'Preview complete. Nothing was installed or changed.'
}

function Write-Section([string]$Text) {
    $step = switch -Regex ($Text) {
        '^Checking installed' { 1; break }
        '^Checking OpenAI|^Checking available' { 2; break }
        '^Checking for a previously|^Downloading' { 4; break }
        '^Validating|^Checking prerequisites' { 5; break }
        '^Closing' { 6; break }
        '^Creating the optional' { 7; break }
        '^Installing' { 8; break }
        '^Verifying installation' { 9; break }
        default { 0 }
    }
    if ($step -gt 0) { Set-UiStep -Number $step -Detail $Text }
    Write-UiMessage ''
    Write-UiMessage "== $Text ==" -ForegroundColor Cyan
}

function Read-YesNo([string]$Prompt) {
    while ($true) {
        # Remove the live panel while Read-Host owns the input line.
        Hide-UiPanel
        $answer = ([string](Read-Host "$Prompt [Y/N]")).Trim()
        Show-UiPanel
        if ($answer -match '(?i)^(y|yes)$') { return $true }
        if ($answer -match '(?i)^(n|no)$')  { return $false }
        Write-UiMessage 'Please enter Y or N.' -ForegroundColor Yellow
    }
}

function Get-JsonObject([string]$Uri) {
    $raw = Invoke-WebRequest `
        -Uri $Uri `
        -Headers @{
            'Cache-Control' = 'no-cache'
            'Pragma'        = 'no-cache'
            'User-Agent'    = "ChatGPT-Corp-Updater/$UpdaterVersion"
        } `
        -UseBasicParsing `
        -TimeoutSec 60

    return ($raw.Content | ConvertFrom-Json)
}

function Get-ObjectPropertyValue {
    param(
        [Parameter(Mandatory = $true)] [AllowNull()] $Object,
        [Parameter(Mandatory = $true)] [string] $Name
    )

    if ($null -eq $Object) {
        return $null
    }

    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) {
        return $null
    }

    return $property.Value
}

function ConvertTo-CurlArgument([string]$Value) {
    # Values here are fixed flags, a validated HTTPS URI, or a Windows filename.
    # No shell is involved. Disallow quote/control injection instead of guessing
    # escaping rules across Windows PowerShell 5.1 and PowerShell 7.
    if ($Value -match '["\x00\r\n]') { throw 'Invalid character in a download argument.' }
    if ($Value.EndsWith('\')) { throw 'Unexpected trailing backslash in a download argument.' }
    return ('"{0}"' -f $Value)
}

function Download-File {
    param([string]$Uri, [string]$Destination, [long]$ExpectedSize = 0)
    $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
    $showTransfer = ($ExpectedSize -gt 0)

    if ($null -ne $curl) {
        # Keep the existing HTTPS, redirect, connection-timeout and retry rules.
        # Silence curl's transfer table, NOT its errors. File bytes drive the UI;
        # there is no parsing of curl's locale-dependent progress output.
        $arguments = @(
            '--location', '--proto', '=https', '--proto-redir', '=https',
            '--connect-timeout', '30', '--fail', '--retry', '3',
            '--retry-delay', '2', '--silent', '--show-error',
            '--output', $Destination, $Uri
        )
        $startInfo = New-Object System.Diagnostics.ProcessStartInfo
        $startInfo.FileName = $curl.Source
        $startInfo.Arguments = (($arguments | ForEach-Object { ConvertTo-CurlArgument $_ }) -join ' ')
        $startInfo.UseShellExecute = $false
        $startInfo.CreateNoWindow = $true
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true
        $process = New-Object System.Diagnostics.Process
        $process.StartInfo = $startInfo
        $started = $false
        $watch = [Diagnostics.Stopwatch]::StartNew()
        $sampleTime = 0.0
        $sampleBytes = 0L
        $speed = 0.0
        $lastBytes = 0L
        try {
            $started = $process.Start()
            if (-not $started) { throw 'Could not start curl.exe.' }
            # Drain both streams asynchronously so a full pipe cannot deadlock.
            $outputTask = $process.StandardOutput.ReadToEndAsync()
            $errorTask = $process.StandardError.ReadToEndAsync()
            if ($showTransfer) {
                $script:Ui.LastBucket = -1
                Update-UiTransfer -Received 0 -Expected $ExpectedSize -BytesPerSecond 0
            }
            while (-not $process.WaitForExit(250)) {
                if (-not $showTransfer) { continue }
                $bytes = 0L
                $fileInfo = Get-Item -LiteralPath $Destination -ErrorAction SilentlyContinue
                if ($null -ne $fileInfo) { $bytes = [long]$fileInfo.Length }
                $now = $watch.Elapsed.TotalSeconds
                if ($bytes -lt $lastBytes) {
                    # curl may truncate a partial file before retrying.
                    $sampleTime = $now; $sampleBytes = $bytes; $speed = 0.0
                }
                if (($now - $sampleTime) -ge 1.0) {
                    $speed = [Math]::Max(0.0, ($bytes - $sampleBytes) / ($now - $sampleTime))
                    $sampleTime = $now; $sampleBytes = $bytes
                }
                $lastBytes = $bytes
                Update-UiTransfer -Received $bytes -Expected $ExpectedSize -BytesPerSecond $speed
            }
            $process.WaitForExit()
            $nativeErrors = $errorTask.GetAwaiter().GetResult()
            $null = $outputTask.GetAwaiter().GetResult()
            if ($process.ExitCode -ne 0) {
                throw ('Download failed. curl.exe exit code: {0}. {1}' -f $process.ExitCode, $nativeErrors.Trim())
            }
            if (-not [string]::IsNullOrWhiteSpace($nativeErrors)) {
                Write-UiMessage $nativeErrors.Trim() -ForegroundColor Yellow
            }
            if ($showTransfer) {
                $readyFile = Get-Item -LiteralPath $Destination -ErrorAction Stop
                # A wrong-length file must not be shown as a completed download.
                Update-UiTransfer -Received $readyFile.Length -Expected $ExpectedSize -BytesPerSecond $speed -Completed:($readyFile.Length -eq $ExpectedSize)
            }
        }
        finally {
            $watch.Stop()
            if ($started) {
                try {
                    if (-not $process.HasExited) {
                        # Cancellation stops only the curl process started here.
                        $process.Kill()
                        [void]$process.WaitForExit(5000)
                    }
                }
                catch { }
            }
            $process.Dispose()
        }
        return
    }

    # Preserve the original web-request fallback on PCs without curl.exe.
    # It has no byte callbacks here, so the current step stays indeterminate
    # instead of fabricating a percentage or ETA.
    if ($showTransfer) {
        $script:Ui.TransferBar = 'Downloading... (curl.exe unavailable; live byte count unavailable)'
        $script:Ui.Transfer = ''
        Show-UiPanel
        if (-not $script:Ui.Live) { Write-UiMessage $script:Ui.TransferBar }
    }
    Invoke-WebRequest `
        -Uri $Uri `
        -OutFile $Destination `
        -Headers @{ 'User-Agent' = "ChatGPT-Corp-Updater/$UpdaterVersion" } `
        -UseBasicParsing
    if ($showTransfer) {
        $readyFile = Get-Item -LiteralPath $Destination -ErrorAction Stop
        Update-UiTransfer -Received $readyFile.Length -Expected $ExpectedSize -BytesPerSecond 0 -Completed:($readyFile.Length -eq $ExpectedSize)
    }
}

function Initialize-AppxCommands {
    # Appx is a Windows PowerShell module on some Windows images. When necessary,
    # use the built-in 5.1 compatibility session from PowerShell 7.
    if ($PSVersionTable.PSVersion.Major -ge 7) {
        Import-Module Appx -UseWindowsPowerShell -Global -ErrorAction Stop -WarningAction SilentlyContinue
    }
    else {
        Import-Module Appx -Global -ErrorAction Stop
    }
    foreach ($command in @('Get-AppxPackage', 'Add-AppxPackage')) {
        if ($null -eq (Get-Command $command -ErrorAction SilentlyContinue)) {
            throw "Required Windows command is unavailable: $command"
        }
    }
}

function Assert-MirrorDownloadUrl([string]$Url) {
    $parsed = [uri]$Url
    if (-not $parsed.IsAbsoluteUri -or $parsed.Scheme -ne 'https' -or
        $parsed.Host -ne 'github.com' -or -not $parsed.IsDefaultPort -or
        -not [string]::IsNullOrEmpty($parsed.UserInfo) -or
        -not $parsed.AbsolutePath.StartsWith("/$MirrorRepo/releases/download/", [StringComparison]::Ordinal)) {
        throw "Unexpected mirror release URL: $Url"
    }
}

function Get-ManifestPackageFamily {
    param(
        [Parameter(Mandatory = $true)] [string]$Name,
        [Parameter(Mandatory = $true)] [string]$Publisher,
        [Parameter(Mandatory = $true)] [string]$Version
    )
    # Let Windows derive the publisher ID; do NOT trust the download filename.
    # This checks identity, not signature validity. Windows validates trust at
    # deployment, and this script never uses -AllowUnsigned.
    if ($null -eq ('CodexCorpUpdaterV51.PackageIdentity' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;
namespace CodexCorpUpdaterV51 {
    public static class PackageIdentity {
        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        private struct PACKAGE_ID {
            public uint reserved;
            public uint processorArchitecture;
            public ulong version;
            [MarshalAs(UnmanagedType.LPWStr)] public string name;
            [MarshalAs(UnmanagedType.LPWStr)] public string publisher;
            [MarshalAs(UnmanagedType.LPWStr)] public string resourceId;
            [MarshalAs(UnmanagedType.LPWStr)] public string publisherId;
        }
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, ExactSpelling = true)]
        private static extern int PackageFamilyNameFromId(
            ref PACKAGE_ID packageId, ref uint length, StringBuilder familyName);

        public static string FromManifest(string name, string publisher, string version) {
            // Package family name depends on package Name + Publisher.
            // Version and architecture do not participate in the resulting PFN,
            // so avoid version bit-packing entirely. This also keeps the helper
            // compatible with the older C# compiler used by Windows PowerShell 5.1.
            PACKAGE_ID id = new PACKAGE_ID();
            id.processorArchitecture = 0;
            id.version = 0UL;
            id.name = name;
            id.publisher = publisher;
            id.resourceId = "";
            id.publisherId = null; // Must be derived from publisher, not supplied by the mirror.
            uint length = 0;
            int result = PackageFamilyNameFromId(ref id, ref length, null);
            if (result != 122 || length == 0 || length > 1024)
                throw new Win32Exception(result, "Cannot determine the MSIX package family.");
            StringBuilder familyName = new StringBuilder((int)length);
            result = PackageFamilyNameFromId(ref id, ref length, familyName);
            if (result != 0) throw new Win32Exception(result);
            return familyName.ToString();
        }
    }
}
'@ -ErrorAction Stop
    }
    return [CodexCorpUpdaterV51.PackageIdentity]::FromManifest($Name, $Publisher, $Version)
}

function Get-MsixIdentity([string]$Path) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem

    $zip = [System.IO.Compression.ZipFile]::OpenRead($Path)
    try {
        $manifestEntry = $zip.Entries |
            Where-Object { $_.FullName -ieq 'AppxManifest.xml' } |
            Select-Object -First 1

        if ($null -eq $manifestEntry) {
            throw 'AppxManifest.xml was not found in the downloaded MSIX.'
        }

        if ($manifestEntry.Length -gt 2MB) {
            throw 'The embedded MSIX manifest is unexpectedly large.'
        }

        $signatureEntry = $zip.Entries |
            Where-Object { $_.FullName -ieq 'AppxSignature.p7x' } |
            Select-Object -First 1

        if ($null -eq $signatureEntry) {
            throw 'AppxSignature.p7x was not found. Refusing to install the package.'
        }

        $stream = $manifestEntry.Open()
        $reader = New-Object System.IO.StreamReader($stream)
        try {
            $xmlText = $reader.ReadToEnd()
            $xmlSettings = New-Object System.Xml.XmlReaderSettings
            $xmlSettings.DtdProcessing = [System.Xml.DtdProcessing]::Prohibit
            $xmlSettings.XmlResolver = $null
            $textReader = New-Object System.IO.StringReader($xmlText)
            $xmlReader = [System.Xml.XmlReader]::Create($textReader, $xmlSettings)
            try {
                $xml = New-Object System.Xml.XmlDocument
                $xml.XmlResolver = $null
                $xml.Load($xmlReader)
            }
            finally {
                $xmlReader.Dispose()
                $textReader.Dispose()
            }
        }
        finally {
            $reader.Dispose()
            $stream.Dispose()
        }

        $identity = $xml.SelectSingleNode("/*[local-name()='Package']/*[local-name()='Identity']")
        if ($null -eq $identity) {
            throw 'Package Identity was not found in AppxManifest.xml.'
        }

        $dependencies = @(
            foreach ($node in $xml.SelectNodes("/*[local-name()='Package']/*[local-name()='Dependencies']/*[local-name()='PackageDependency']")) {
                $minVersion = $node.GetAttribute('MinVersion')
                if ([string]::IsNullOrEmpty($minVersion)) { $minVersion = '0.0.0.0' }
                [pscustomobject]@{
                    Name       = $node.GetAttribute('Name')
                    Publisher  = $node.GetAttribute('Publisher')
                    MinVersion = [version]$minVersion
                }
            }
        )
        return [pscustomobject]@{
            Name                  = $identity.GetAttribute('Name')
            Publisher             = $identity.GetAttribute('Publisher')
            Version               = $identity.GetAttribute('Version')
            ProcessorArchitecture = $identity.GetAttribute('ProcessorArchitecture')
            Dependencies          = $dependencies
        }
    }
    finally {
        $zip.Dispose()
    }
}

function Get-MirrorCandidatesFromReleases {
    param([AllowEmptyCollection()] [object[]]$Releases = @())

    # Only actual, uploaded Windows x64 stable-family assets are candidates.
    # Release titles/tags can be internal/macOS versions and are NOT compared.
    $pattern = '^OpenAI\.Codex_(\d+\.\d+\.\d+\.\d+)_x64__2p2nqsd0c76g0\.msix$'
    foreach ($release in $Releases) {
        if ($null -eq $release) { continue }
        if ((Get-ObjectPropertyValue -Object $release -Name 'draft') -eq $true) { continue }
        if ((Get-ObjectPropertyValue -Object $release -Name 'prerelease') -eq $true) { continue }
        $assets = @(Get-ObjectPropertyValue -Object $release -Name 'assets')
        $checksumAssets = @(
            $assets | Where-Object {
                (Get-ObjectPropertyValue -Object $_ -Name 'name') -ieq 'SHA256SUMS.txt' -and
                (Get-ObjectPropertyValue -Object $_ -Name 'state') -eq 'uploaded'
            }
        )
        if ($checksumAssets.Count -gt 1) {
            throw 'The mirror release has ambiguous SHA256SUMS.txt assets.'
        }
        $checksumUrl = $null
        if ($checksumAssets.Count -eq 1) {
            $checksumUrl = [string](Get-ObjectPropertyValue -Object $checksumAssets[0] -Name 'browser_download_url')
        }

        foreach ($asset in $assets) {
            $assetName = [string](Get-ObjectPropertyValue -Object $asset -Name 'name')
            $match = [regex]::Match($assetName, $pattern, [Text.RegularExpressions.RegexOptions]::IgnoreCase)
            if (-not $match.Success) { continue }
            if ((Get-ObjectPropertyValue -Object $asset -Name 'state') -ne 'uploaded') { continue }
            $assetVersion = $null
            if (-not [version]::TryParse($match.Groups[1].Value, [ref]$assetVersion)) { continue }
            $parts = @($assetVersion.Major, $assetVersion.Minor, $assetVersion.Build, $assetVersion.Revision)
            if (@($parts | Where-Object { $_ -lt 0 -or $_ -gt 65535 }).Count -gt 0) { continue }
            $assetSize = [int64](Get-ObjectPropertyValue -Object $asset -Name 'size')
            if ($assetSize -le 0) { continue }
            $downloadUrl = [string](Get-ObjectPropertyValue -Object $asset -Name 'browser_download_url')
            Assert-MirrorDownloadUrl -Url $downloadUrl
            if (-not [string]::IsNullOrWhiteSpace($checksumUrl)) {
                Assert-MirrorDownloadUrl -Url $checksumUrl
            }
            [pscustomobject]@{
                Version         = $assetVersion
                ReleaseName     = [string](Get-ObjectPropertyValue -Object $release -Name 'name')
                TagName         = [string](Get-ObjectPropertyValue -Object $release -Name 'tag_name')
                AssetName       = $assetName
                DownloadUrl     = $downloadUrl
                AssetSize       = $assetSize
                ChecksumUrl     = $checksumUrl
            }
        }
    }
}

function Get-MirrorReleaseCatalog {
    # Paginate: GitHub's latest release can be a macOS-only update.
    $candidates = New-Object 'System.Collections.Generic.List[object]'
    $maximumPages = 5
    for ($page = 1; $page -le $maximumPages; $page++) {
        try {
            $pageResponse = Invoke-RestMethod `
                    -Uri "${MirrorApi}?per_page=100&page=$page" `
                    -Headers @{
                        'Accept'        = 'application/vnd.github+json'
                        'User-Agent'    = "ChatGPT-Corp-Updater/$UpdaterVersion"
                        'Cache-Control' = 'no-cache'
                        'Pragma'        = 'no-cache'
                    } `
                    -TimeoutSec 60 `
                    -ErrorAction Stop
            # Assign before enumerating: Invoke-RestMethod can emit a JSON array
            # as a single pipeline object in Windows PowerShell 5.1.
            $releases = @($pageResponse)
        }
        catch {
            throw "Cannot read mirror release metadata (page $page). This is a network/API error, not proof that a version is missing. $($_.Exception.Message)"
        }
        foreach ($candidate in @(Get-MirrorCandidatesFromReleases -Releases $releases)) {
            $candidates.Add($candidate)
        }
        if ($releases.Count -lt 100) { break }
        if ($page -eq $maximumPages) {
            Write-UiMessage 'Search scope       : 500 most recent releases; older releases were not scanned.' -ForegroundColor Yellow
        }
    }
    # Cast to Version: sorting strings would rank e.g. 9818 above 10000.
    $candidates | Sort-Object { [version]$_.Version } -Descending
}

function Select-TargetMirrorRelease {
    param(
        [Parameter(Mandatory = $true)] [version]$AdvertisedVersion,
        [AllowNull()] [version]$InstalledVersion = $null,
        [AllowEmptyCollection()] [object[]]$Candidates = @(),
        [switch]$ExactOnly
    )
    # Pure selection: no network, prompts, files or installation here.
    # Candidates have already passed the stable-family/x64/uploaded/non-preview
    # metadata filter. Actual package identity/trust is checked before deployment.
    # Never use the feed as a ceiling in normal mode, or an ahead-of-feed install
    # would incorrectly stop receiving future mirror updates.
    $eligible = @(
        $Candidates | Where-Object {
            $null -ne $_ -and
            ($null -eq $InstalledVersion -or [version]$_.Version -gt $InstalledVersion) -and
            (-not $ExactOnly -or [version]$_.Version -eq $AdvertisedVersion)
        } | Sort-Object { [version]$_.Version } -Descending
    )
    if ($eligible.Count -eq 0) { return $null }
    return $eligible[0]
}

function Get-InstallConfirmationPrompt {
    param(
        [Parameter(Mandatory = $true)] [version]$TargetVersion,
        [Parameter(Mandatory = $true)] [version]$AdvertisedVersion,
        [switch]$IsUpgrade
    )
    # Format strings keep question marks outside PowerShell variable names.
    # Every mismatch is named in the user's actual confirmation question.
    if ($TargetVersion -gt $AdvertisedVersion) {
        return ('Install mirrored {0}, NEWER than OpenAI feed {1}?' -f $TargetVersion, $AdvertisedVersion)
    }
    if ($TargetVersion -lt $AdvertisedVersion) {
        return ('Install available {0} instead of advertised {1}?' -f $TargetVersion, $AdvertisedVersion)
    }
    if ($IsUpgrade) { return ('Install update {0}?' -f $TargetVersion) }
    return ('Install ChatGPT/Codex {0} for this Windows user?' -f $TargetVersion)
}

function Get-MissingDependencies($Identity) {
    foreach ($dependency in $Identity.Dependencies) {
        $found = @(
            Get-AppxPackage -Name $dependency.Name -ErrorAction Stop |
                Where-Object {
                    [version]$_.Version -ge $dependency.MinVersion -and
                    [string]$_.Architecture -in @('X64', 'Neutral') -and
                    ([string]::IsNullOrEmpty($dependency.Publisher) -or
                     [string]$_.Publisher -ceq $dependency.Publisher)
                }
        )
        if ($found.Count -eq 0) { Write-Output $dependency }
    }
}

function Get-TargetAppProcesses([string]$InstallLocation) {
    if ([string]::IsNullOrWhiteSpace($InstallLocation)) { return }
    $prefix = $InstallLocation.TrimEnd([char[]]@('\', '/')) + '\'
    $session = [System.Diagnostics.Process]::GetCurrentProcess().SessionId
    foreach ($process in @(Get-Process -ErrorAction Stop)) {
        try {
            if ($process.Id -eq $PID -or $process.SessionId -ne $session) { continue }
            $path = [string]$process.Path
            if (-not [string]::IsNullOrWhiteSpace($path) -and
                $path.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
                Write-Output $process
            }
        }
        catch {
            # Access to unrelated/protected processes may be denied. Do not kill
            # a process unless its executable can be tied to this installed app.
        }
    }
}

function Close-TargetApp([string]$InstallLocation) {
    $processes = @(Get-TargetAppProcesses -InstallLocation $InstallLocation)
    if ($processes.Count -eq 0) { return }
    Write-UiMessage 'Closing processes belonging to this ChatGPT/Codex package.'
    foreach ($process in $processes) {
        try { [void]$process.CloseMainWindow() } catch { }
    }
    Start-Sleep -Seconds 3
    foreach ($process in @(Get-TargetAppProcesses -InstallLocation $InstallLocation)) {
        try { $process.Kill() }
        catch {
            if (-not $process.HasExited) {
                throw 'Cannot close the app. Finish its tasks, quit ChatGPT/Codex, and run this script again.'
            }
        }
    }
    Start-Sleep -Seconds 1
    if (@(Get-TargetAppProcesses -InstallLocation $InstallLocation).Count -gt 0) {
        throw 'ChatGPT/Codex is still running. Quit the app and run the script again.'
    }
}

function Backup-CodexState {
    $codexState = Join-Path $HOME '.codex'
    if (-not (Test-Path -LiteralPath $codexState)) {
        Write-UiMessage 'No ~/.codex directory found; skipping backup.'
        return $null
    }

    $backupRoot = Join-Path $HOME ("CodexBackups\{0}" -f (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
    New-Item -ItemType Directory -Path $backupRoot -Force | Out-Null

    Copy-Item `
        -LiteralPath $codexState `
        -Destination $backupRoot `
        -Recurse `
        -Force

    return $backupRoot
}

try {
    Write-UiMessage "ChatGPT/Codex Installer + Updater v$UpdaterVersion" -ForegroundColor Green
    Write-UiMessage "PowerShell         : $($PSVersionTable.PSVersion)"
    Write-UiMessage 'Runs for the current user; Microsoft Store client and WinGet are not used.'

    Initialize-Ui
    if ($Preview) {
        Invoke-UiPreview
        exit 0
    }
    Set-UiStep 1 'Checking this PC and loading AppX commands'

    if ($Backup -and $NoBackup) {
        throw 'Choose -Backup OR -NoBackup, not both. Omit both for a backup prompt on upgrades.'
    }
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
        throw 'This script must run on Windows.'
    }
    if (-not [Environment]::Is64BitOperatingSystem -or -not [Environment]::Is64BitProcess) {
        throw 'Run the script in 64-bit Windows PowerShell 5.1 or 64-bit PowerShell 7.'
    }
    $nativeArchitecture = $env:PROCESSOR_ARCHITEW6432
    if ([string]::IsNullOrWhiteSpace($nativeArchitecture)) {
        $nativeArchitecture = $env:PROCESSOR_ARCHITECTURE
    }
    if ($nativeArchitecture -ne 'AMD64') {
        throw 'This version of the script supports Windows x64 (Intel/AMD), not ARM64 or x86.'
    }

    # This is a per-process TLS setting, not a machine-policy change.
    [Net.ServicePointManager]::SecurityProtocol =
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $stage = 'Loading Windows AppX commands'
    Initialize-AppxCommands

    $resolvedDependencyPaths = @(
        foreach ($dependencyFile in $DependencyPath) {
            $item = Get-Item -LiteralPath $dependencyFile -ErrorAction Stop
            if ($item.PSIsContainer -or $item.Extension -notin @('.appx', '.msix', '.appxbundle', '.msixbundle')) {
                throw "Not a dependency package or bundle: $dependencyFile"
            }
            $item.FullName
        }
    )

    $stage = 'Checking installed package'
    Write-Section $stage
    $installed = Get-AppxPackage -Name $PackageName -ErrorAction Stop |
        Sort-Object { [version]$_.Version } -Descending |
        Select-Object -First 1
    $isUpgrade = ($null -ne $installed)
    $installedVersion = $null
    $doBackup = $false

    if ($isUpgrade) {
        if ([string]$installed.PackageFamilyName -cne $ExpectedPackageFamily) {
            throw "Unsupported package family: $($installed.PackageFamilyName). Expected $ExpectedPackageFamily."
        }
        $installedVersion = [version]$installed.Version
        Write-UiMessage 'Mode               : Upgrade existing installation'
        Write-UiMessage "Installed version  : $installedVersion"
        Write-UiMessage "Package            : $($installed.PackageFullName)"
    }
    else {
        Write-UiMessage 'Mode               : Fresh installation'
        Write-UiMessage 'Installed version  : Not installed for the current Windows user'
        Write-UiMessage 'Other app families (including Beta) will not be removed or migrated.'
        Skip-UiStep 6
        Skip-UiStep 7
    }

    $stage = 'Checking OpenAI for the latest advertised Windows build'
    Write-Section $stage
    $cacheBust = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $manifest = Get-JsonObject -Uri "${OpenAIManifestUrl}?ts=$cacheBust"
    $buildVersionValue = Get-ObjectPropertyValue -Object $manifest -Name 'buildVersion'
    $buildVersionText = [string]$buildVersionValue
    if ($buildVersionText -notmatch '^\d+\.\d+\.\d+\.\d+$') {
        throw "OpenAI's metadata is missing a valid four-part buildVersion. Received: '$buildVersionText'. No version or download URL was guessed."
    }
    $advertisedVersion = [version]$buildVersionText
    foreach ($part in @($advertisedVersion.Major, $advertisedVersion.Minor, $advertisedVersion.Build, $advertisedVersion.Revision)) {
        if ($part -gt 65535) { throw 'OpenAI returned an invalid Windows package version.' }
    }
    # Validate identity when supplied; do not require fields the feed may omit.
    $feedIdentity = [string](Get-ObjectPropertyValue -Object $manifest -Name 'packageIdentity')
    $feedProduct = [string](Get-ObjectPropertyValue -Object $manifest -Name 'storeProductId')
    if (-not [string]::IsNullOrWhiteSpace($feedIdentity) -and $feedIdentity -cne $PackageName) {
        throw "Unexpected package identity in OpenAI's feed: $feedIdentity"
    }
    if (-not [string]::IsNullOrWhiteSpace($feedProduct) -and $feedProduct -cne '9PLM9XGG6VKS') {
        throw "Unexpected Store product in OpenAI's feed: $feedProduct"
    }
    Write-UiMessage "Advertised version : $advertisedVersion"

    if ($isUpgrade -and $advertisedVersion -le $installedVersion) {
        Write-UiMessage 'Installed version already meets/exceeds this feed; still checking mirror availability.'
    }

    $stage = 'Checking available Windows MSIX packages'
    Write-Section $stage
    Write-UiMessage "Package source     : third-party GitHub mirror $MirrorRepo" -ForegroundColor Yellow
    Write-UiMessage 'The package must match the stable OpenAI.Codex identity; Windows validates its signature.'
    $catalog = @(Get-MirrorReleaseCatalog)
    if ($catalog.Count -eq 0) {
        Write-UiMessage 'No uploaded stable x64 MSIX assets were found in the scanned mirror releases.' -ForegroundColor Yellow
        Write-UiMessage 'No installer was downloaded, no app was closed, and no package was changed.'
        Finish-UiSteps 'No installation performed; no eligible update selected.'
        exit 0
    }
    Write-UiMessage "Newest mirror MSIX : $($catalog[0].Version)"
    if ([version]$catalog[0].Version -gt $advertisedVersion) {
        if ($ExactVersionOnly) {
            Write-UiMessage 'Mirror is ahead of the feed. -ExactVersionOnly still requires the exact feed build.' -ForegroundColor Yellow
        }
        else {
            Write-UiMessage 'Mirror is ahead of the feed. A newer build may be offered, with an explicit warning and Y/N confirmation.' -ForegroundColor Yellow
        }
    }
    $mirrorRelease = Select-TargetMirrorRelease `
        -AdvertisedVersion $advertisedVersion `
        -InstalledVersion $installedVersion `
        -Candidates $catalog `
        -ExactOnly:$ExactVersionOnly
    if ($null -eq $mirrorRelease) {
        if ($ExactVersionOnly -and $isUpgrade -and $installedVersion -ge $advertisedVersion) {
            Write-UiMessage 'The installed version already meets/exceeds the advertised build. No downgrade or reinstall will be performed.' -ForegroundColor Green
        }
        elseif ($ExactVersionOnly) {
            Write-UiMessage "Advertised build $advertisedVersion was not found as an eligible mirror MSIX. -ExactVersionOnly prevents selecting another build." -ForegroundColor Yellow
        }
        elseif ($isUpgrade) {
            Write-UiMessage 'No compatible mirrored build is newer than your installed package. No downgrade or reinstall will be performed.' -ForegroundColor Green
        }
        else {
            Write-UiMessage 'No compatible mirror MSIX is available for a fresh installation.' -ForegroundColor Yellow
        }
        Write-UiMessage 'This result concerns the scanned mirror assets, not every official distribution channel.'
        Write-UiMessage 'No installer was downloaded, no app was closed, and no package was changed.'
        Finish-UiSteps 'No installation performed; no eligible update selected.'
        exit 0
    }

    $targetVersion = [version]$mirrorRelease.Version
    if ($isUpgrade -and $targetVersion -le $installedVersion) {
        throw 'Internal selection error: refusing to downgrade or reinstall the existing package.'
    }
    $isFallback = ($targetVersion -lt $advertisedVersion)
    $isAheadOfFeed = ($targetVersion -gt $advertisedVersion)
    Assert-MirrorDownloadUrl -Url $mirrorRelease.DownloadUrl
    if (-not [string]::IsNullOrWhiteSpace([string]$mirrorRelease.ChecksumUrl)) {
        Assert-MirrorDownloadUrl -Url $mirrorRelease.ChecksumUrl
    }
    Write-UiMessage "Selected version   : $targetVersion"
    Write-UiMessage "Mirror release     : $($mirrorRelease.ReleaseName)"
    Write-UiMessage "MSIX asset         : $($mirrorRelease.AssetName)"
    if ($isAheadOfFeed) {
        Write-UiMessage ''
        Write-UiMessage "Mirror package $targetVersion is NEWER than the OpenAI feed build $advertisedVersion." -ForegroundColor Yellow
        Write-UiMessage 'This feed does NOT confirm the newer build. A higher version alone does not prove authenticity or stability.' -ForegroundColor Yellow
        Write-UiMessage 'Only proceed if your organization permits this mirrored package. Identity and Windows signature checks still apply.' -ForegroundColor Yellow
    }
    elseif ($isFallback) {
        Write-UiMessage ''
        Write-UiMessage "OpenAI advertises $advertisedVersion, but that MSIX was not found in the scanned mirror releases." -ForegroundColor Yellow
        Write-UiMessage "Available alternative: $targetVersion. This is NOT the latest advertised build." -ForegroundColor Yellow
        if ($isUpgrade) {
            Write-UiMessage "It is an upgrade from your installed $installedVersion, not a downgrade."
        }
    }
    if ($CheckOnly) {
        Write-UiMessage 'Check only: no installer downloaded, no backup, and no installation.'
        Finish-UiSteps 'Check only complete. No changes made.'
        exit 0
    }

    # Prompt AFTER resolving the actual target. Consent to an advertised build
    # is not consent to a different build. Backup is also asked only afterward.
    $stage = 'Confirming installation'
    Set-UiStep 3 'Waiting for your Y/N choices'
    Write-UiMessage ''
    if ($isUpgrade) {
        Write-UiMessage "AVAILABLE UPGRADE: $installedVersion -> $targetVersion" -ForegroundColor Green
        Write-UiMessage 'Finish running tasks and save work. Updating will close this ChatGPT/Codex app.'
    }
    else {
        Write-UiMessage "READY TO INSTALL: $PackageName $targetVersion" -ForegroundColor Green
    }
    $installPrompt = Get-InstallConfirmationPrompt `
        -TargetVersion $targetVersion `
        -AdvertisedVersion $advertisedVersion `
        -IsUpgrade:$isUpgrade
    $approved = Read-YesNo $installPrompt
    if (-not $approved) {
        Write-UiMessage 'Cancelled. No packages, backups or installers were written.'
        Finish-UiSteps 'Cancelled by user. No changes made.'
        exit 0
    }

    # Backup remains optional, only for upgrades, and never runs before consent.
    if ($isUpgrade) {
        if ($NoBackup) {
            Write-UiMessage 'Backup             : Disabled by -NoBackup'
        }
        elseif ($Backup) {
            $doBackup = $true
            Write-UiMessage 'Backup             : Enabled by -Backup'
        }
        else {
            $doBackup = Read-YesNo 'Back up your .codex folder before upgrading?'
            if (-not $doBackup) { Write-UiMessage 'Backup             : Skipped by your choice' }
        }
    }
    else {
        Write-UiMessage 'Backup             : Not created for a fresh installation'
    }

    if (-not $doBackup -and $script:Ui.Steps[6].State -ne 'Skipped') { Skip-UiStep 7 }
    Set-UiStep 4 ('Preparing installer {0}; checking metadata and cache' -f $targetVersion)

    $workDir = Join-Path $env:TEMP ("ChatGPT-CorpUpdater-v5.4\{0}-{1}" -f $targetVersion, [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $workDir -Force | Out-Null
    $downloadTarget = Join-Path $workDir $mirrorRelease.AssetName
    $checksumsPath = Join-Path $workDir 'SHA256SUMS.txt'
    $expectedSha256 = $null

    # The checksum is tiny, so fetch it first. This lets us safely reuse a
    # previously downloaded 700+ MB MSIX from an earlier failed run.
    if (-not [string]::IsNullOrWhiteSpace([string]$mirrorRelease.ChecksumUrl)) {
        $stage = 'Downloading checksum metadata'
        Download-File -Uri $mirrorRelease.ChecksumUrl -Destination $checksumsPath

        $checksumPattern = '^([A-Fa-f0-9]{64})\s+\*?' + [regex]::Escape($mirrorRelease.AssetName) + '\s*$'
        $expectedHashes = @(
            foreach ($line in (Get-Content -LiteralPath $checksumsPath)) {
                if ($line -match $checksumPattern) { $Matches[1].ToLowerInvariant() }
            }
        )
        if ($expectedHashes.Count -ne 1) {
            throw 'SHA256 checksum entry for the target MSIX is missing or ambiguous.'
        }
        $expectedSha256 = $expectedHashes[0]
    }

    $stage = 'Checking for a previously downloaded MSIX'
    Write-Section $stage

    $reused = $false
    $msixPath = $null
    $reuseRoots = @(
        (Join-Path $env:TEMP 'ChatGPT-CorpUpdater-v5'),
        (Join-Path $env:TEMP 'ChatGPT-CorpUpdater-v5.1'),
        (Join-Path $env:TEMP 'ChatGPT-CorpUpdater-v5.2'),
        (Join-Path $env:TEMP 'ChatGPT-CorpUpdater-v5.3'),
        (Join-Path $env:TEMP 'ChatGPT-CorpUpdater-v5.4')
    ) | Select-Object -Unique

    if ([string]::IsNullOrWhiteSpace([string]$expectedSha256)) {
        Write-UiMessage 'Cache reuse disabled: this release has no published checksum.' -ForegroundColor Yellow
        $reuseRoots = @()
    }
    foreach ($reuseRoot in $reuseRoots) {
        if (-not (Test-Path -LiteralPath $reuseRoot)) { continue }

        $candidates = @(
            Get-ChildItem -LiteralPath $reuseRoot -Recurse -File -ErrorAction SilentlyContinue |
                Where-Object {
                    $_.Name -ieq $mirrorRelease.AssetName -or
                    ($_.Extension -ieq '.msix' -and $_.FullName -match [regex]::Escape([string]$targetVersion))
                } |
                Sort-Object LastWriteTime -Descending
        )

        foreach ($candidate in $candidates) {
            if ($candidate.FullName -eq $downloadTarget) { continue }
            if ($candidate.Length -lt 10MB) { continue }
            if ($mirrorRelease.AssetSize -gt 0 -and $candidate.Length -ne $mirrorRelease.AssetSize) { continue }

            try {
                $candidateHash = (Get-FileHash -LiteralPath $candidate.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            }
            catch {
                continue
            }

            if (-not [string]::IsNullOrWhiteSpace([string]$expectedSha256) -and
                $candidateHash -cne $expectedSha256) {
                continue
            }

            $msixPath = $candidate.FullName
            $reused = $true
            Write-UiMessage "Reusing existing  : $msixPath" -ForegroundColor Green
            Write-UiMessage ("Existing size     : {0:N1} MB" -f ($candidate.Length / 1MB))
            Write-UiMessage "SHA256             : $candidateHash"
            break
        }

        if ($reused) { break }
    }

    if (-not $reused) {
        $stage = "Downloading $targetVersion"
        Write-Section $stage
        $msixPath = $downloadTarget
        Download-File -Uri $mirrorRelease.DownloadUrl -Destination $msixPath -ExpectedSize $mirrorRelease.AssetSize
    }

    $downloadedFile = Get-Item -LiteralPath $msixPath
    if ($downloadedFile.Length -lt 10MB) {
        throw "MSIX is unexpectedly small ($($downloadedFile.Length) bytes)."
    }
    if ($mirrorRelease.AssetSize -gt 0 -and $downloadedFile.Length -ne $mirrorRelease.AssetSize) {
        throw 'MSIX file size does not match the GitHub release asset size.'
    }

    if (-not $reused) {
        Write-UiMessage ("Downloaded         : {0:N1} MB" -f ($downloadedFile.Length / 1MB))
    }

    $stage = 'Checking SHA-256 integrity'
    Set-UiStep 5 $stage
    $actualSha256 = (Get-FileHash -LiteralPath $msixPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if (-not $reused) {
        Write-UiMessage "SHA256             : $actualSha256"
    }

    if (-not [string]::IsNullOrWhiteSpace([string]$expectedSha256)) {
        if ($expectedSha256 -cne $actualSha256) {
            throw 'SHA256 verification failed. The MSIX will not be installed.'
        }
        Write-UiMessage 'Mirror checksum    : Matches (integrity check, not an independent trust source)' -ForegroundColor Green
    }
    else {
        Write-UiMessage 'Mirror checksum    : Not published; Windows signature validation is still required.' -ForegroundColor Yellow
    }

    $stage = 'Validating package identity'
    Write-Section $stage
    $identity = Get-MsixIdentity -Path $msixPath
    if ($identity.Name -cne $PackageName) {
        throw "Wrong package: $($identity.Name). Expected $PackageName."
    }
    if ([version]$identity.Version -ne $targetVersion) {
        throw "MSIX version $($identity.Version) does not match the selected version $targetVersion (OpenAI advertises $advertisedVersion)."
    }
    if ($identity.ProcessorArchitecture -notin @('x64', 'neutral')) {
        throw "Wrong package architecture: $($identity.ProcessorArchitecture)."
    }
    $downloadedFamily = Get-ManifestPackageFamily -Name $identity.Name -Publisher $identity.Publisher -Version $identity.Version
    if ($downloadedFamily -cne $ExpectedPackageFamily) {
        throw "Package family mismatch: $downloadedFamily. Expected $ExpectedPackageFamily."
    }
    if ($isUpgrade -and $identity.Publisher -cne [string]$installed.Publisher) {
        throw 'Package publisher does not match the currently installed app.'
    }
    Write-UiMessage "Package family     : $downloadedFamily"
    Write-UiMessage "Package version    : $($identity.Version)"
    Write-UiMessage 'Identity check     : OK (also checked for fresh installation)' -ForegroundColor Green
    Write-UiMessage 'Embedded signature : Present; cryptographic trust is checked by Windows at installation.'

    $stage = 'Checking prerequisites'
    Write-Section $stage
    $missingDependencies = @(Get-MissingDependencies -Identity $identity)
    if ($missingDependencies.Count -gt 0) {
        Write-UiMessage 'Required frameworks not registered at a sufficient version:' -ForegroundColor Yellow
        foreach ($dependency in $missingDependencies) {
            Write-UiMessage "  $($dependency.Name), minimum $($dependency.MinVersion), x64/neutral"
        }
        if ($resolvedDependencyPaths.Count -eq 0) {
            throw 'Missing Windows framework dependencies. Obtain the listed signed packages from Microsoft or your IT team, then rerun with -DependencyPath (see Get-Help on this script). No app package was installed and no backup was made.'
        }
        Write-UiMessage 'Windows will resolve these prerequisites using your supplied dependency packages.'
    }
    else {
        Write-UiMessage 'Declared framework prerequisites are present.'
    }

    # Recheck the package just before changing anything in case another updater ran.
    $current = Get-AppxPackage -Name $PackageName -ErrorAction Stop |
        Sort-Object { [version]$_.Version } -Descending | Select-Object -First 1
    if ($null -ne $current -and [version]$current.Version -ge $targetVersion -and
        [string]$current.PackageFamilyName -ceq $ExpectedPackageFamily) {
        Write-UiMessage 'Another process already installed this build or a newer one. No installation is needed.'
        Finish-UiSteps 'Another updater already installed this version; no deployment performed here.'
        Remove-Item -LiteralPath $workDir -Recurse -Force
        exit 0
    }
    if (($isUpgrade -and ($null -eq $current -or $current.PackageFullName -cne $installed.PackageFullName)) -or
        (-not $isUpgrade -and $null -ne $current)) {
        throw 'The installed package changed during the download. Rerun the script to choose the correct install/backup flow.'
    }

    if ($isUpgrade) {
        $stage = 'Closing the existing app'
        Write-Section $stage
        Close-TargetApp -InstallLocation ([string]$installed.InstallLocation)
        # Back up AFTER closing the app, not while it is writing its state.
        if ($doBackup) {
            $stage = 'Creating the optional backup'
            Write-Section $stage
            $backupPath = Backup-CodexState
            if ($null -ne $backupPath) { Write-UiMessage "Backup saved       : $backupPath" }
            else { Skip-UiStep 7 }
        }
    }

    $stage = 'Installing the signed package'
    Write-Section $stage
    $installArguments = @{ Path = $msixPath; ErrorAction = 'Stop' }
    if ($isUpgrade) {
        $addCommand = Get-Command Add-AppxPackage -ErrorAction Stop
        if ($addCommand.Parameters.ContainsKey('ForceTargetApplicationShutdown')) {
            $installArguments['ForceTargetApplicationShutdown'] = $true
        }
        # On older deployments the app has already been closed above. Do not
        # force-close every app that shares its runtime dependencies.
    }
    if ($resolvedDependencyPaths.Count -gt 0) {
        $installArguments['DependencyPath'] = $resolvedDependencyPaths
    }
    # This command supports BOTH a new per-user install and an in-place update.
    # Do not add -AllowUnsigned, certificate imports, or policy modifications.
    Add-AppxPackage @installArguments

    $stage = 'Verifying installation'
    Write-Section $stage
    $updated = Get-AppxPackage -Name $PackageName -ErrorAction Stop |
        Sort-Object { [version]$_.Version } -Descending |
        Select-Object -First 1
    if ($null -eq $updated) { throw "$PackageName is not registered after installation." }
    if ([string]$updated.PackageFamilyName -cne $ExpectedPackageFamily) {
        throw 'The registered package family is not the expected stable app.'
    }
    if ([version]$updated.Version -lt $targetVersion) {
        throw "Expected version $targetVersion or newer, but Windows reports $($updated.Version)."
    }
    Write-UiMessage ''
    if ($isUpgrade) {
        Write-UiMessage "SUCCESS: ChatGPT/Codex updated to $($updated.Version)." -ForegroundColor Green
    }
    else {
        Write-UiMessage "SUCCESS: ChatGPT/Codex installed: $($updated.Version)." -ForegroundColor Green
    }
    if ($null -ne $backupPath) { Write-UiMessage "Backup retained    : $backupPath" }
    else { Write-UiMessage 'Backup             : Not created' }
    if ($isFallback -and [version]$updated.Version -lt $advertisedVersion) {
        Write-UiMessage "Installed the available build. OpenAI still advertises $advertisedVersion; this run did not install that advertised build." -ForegroundColor Yellow
    }
    if ($isAheadOfFeed) {
        Write-UiMessage "Installed a mirrored build above the feed version $advertisedVersion read at startup. It was explicitly approved; the feed did not confirm it." -ForegroundColor Yellow
    }
    Write-UiMessage 'Open the app from the Start menu. No Microsoft Store client was used.' 
    Finish-UiSteps ('Verified installed version {0}. Finished.' -f $updated.Version)
    Remove-Item -LiteralPath $workDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 0
}
catch {
    Finish-UiSteps ('Failed: {0}' -f $stage) -Failed
    Write-UiMessage ''
    Write-UiMessage 'INSTALL / UPDATE FAILED' -ForegroundColor Red
    Write-UiMessage "Stage: $stage" -ForegroundColor Yellow
    Write-UiMessage $_.Exception.Message -ForegroundColor Red
    Write-UiMessage ("Script line: {0}" -f $_.InvocationInfo.ScriptLineNumber)
    Write-UiMessage 'The script did not run an uninstall command.'
    if (Get-Variable msixPath -ErrorAction SilentlyContinue) {
        if (-not [string]::IsNullOrWhiteSpace([string]$msixPath)) { Write-UiMessage "MSIX path          : $msixPath" }
    }
    if ($null -ne $workDir -and (Test-Path -LiteralPath $workDir)) {
        Write-UiMessage "Downloaded files retained for troubleshooting: $workDir"
    }
    if ($null -ne $backupPath) { Write-UiMessage "Backup retained: $backupPath" }
    Write-UiMessage 'If Windows reports a policy or licensing restriction, ask IT to approve/deploy the package.'
    exit 1
}
finally {
    Close-Ui
}
