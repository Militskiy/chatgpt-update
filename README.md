# ChatGPT Update — portable Windows application

**v0.2.1 self-update test release. Windows x64.** This is an independent utility, not an OpenAI or Microsoft product. Use only with your organization's approval.

> **The original v0.1.0 EXE has a reported Defender `Trojan:Win32/Wacatac.B!ml` detection. Do not unblock that release.** The cause remains unconfirmed; no Microsoft analyst clearance has been obtained. See [SECURITY.md](SECURITY.md) and the release's Defender report. A local scan is not a guarantee of endpoint/cloud acceptance.

## v0.2.1 update test

The menu now shows `Tip: 1 updates ChatGPT; 4 updates this utility.` Use option **4** or `chatgpt-update self-update` in your permitted v0.2.0 folder, then restart the app and check `chatgpt-update --version` reports `0.2.1`.

Version 0.2.1 is a maintainer-requested exception to prerelease-only publishing: v0.2.0 can discover only normal GitHub releases. Normal-channel metadata is **not** an antivirus clearance. This package remains unsigned, the earlier v0.1.0 detection remains unresolved, and all scan/report requirements remain in force. `release-approval.json` scopes this exception to 0.2.1; future versions default to prerelease unless separately approved.

## What changed in v0.2.0

The app is now a **portable folder**, not a self-extracting single EXE. PowerShell scripts are visible alongside the application, and their exact SHA-256 hashes are bound into the EXE at build time. Missing or altered scripts stop execution. The program no longer embeds/extracts PowerShell code into TEMP, passes `-ExecutionPolicy Bypass`, or alters execution policy. No obfuscator/packer is used and Go debug information is retained.

These are transparency and security improvements, not a claim that we identified Defender's classifier trigger. No antivirus exclusions, security disabling, certificate imports, quarantine restoration or corporate-policy bypasses are implemented.

## Start

Download **chatgpt-update-windows-x64.zip** from the intended release. Extract the **entire** ZIP to a writable permanent folder, for example `%LOCALAPPDATA%\Programs\ChatGPTUpdater`. Do not copy just the EXE.

```text
ChatGPTUpdater\
  chatgpt-update.exe
  VERSION
  FILES.sha256
  README.md
  scripts\
    path.ps1
    prepare-state.ps1
    replace-updater.ps1
    update-chatgpt.ps1
```

Launch `chatgpt-update.exe`, or run from that folder:

```powershell
.\chatgpt-update.exe
```

No MSI, Go, Git, WinGet, PowerShell 7 or Microsoft account is required on the target PC. Windows PowerShell 5.1 and permission to deploy signed AppX packages are required.

**Existing PowerShell execution policy is respected.** Restricted/AllSigned/RemoteSigned policies or downloaded-file markings may block unsigned scripts. The app will stop and direct you to IT; it does not change policy or unblock files. For an organization requiring signed scripts, signing must occur before computing embedded hashes and building the EXE. After that, sign the EXE and scan the final signed distributable. This package is unsigned.

## Menu and commands

```text
1) Check for ChatGPT update / install
2) Create backup
3) Restore backup
4) Update the updater
5) Add this folder to user PATH
0) Exit
```

Option 1 retains updater engine v5.4: installed/feed/mirror versions, explicit Y/N consent, optional upgrade backup, cached MSIX reuse, package identity/publisher checks, and the live checklist. A fresh install has no upgrade backup. The ChatGPT package is stable `OpenAI.Codex_2p2nqsd0c76g0`; downloads still use the third-party `Wangnov/codex-app-mirror`. Windows validates the MSIX during `Add-AppxPackage`; missing approved dependencies or corporate restrictions are reported, not bypassed.

Option 5 adds the current folder to **user PATH**, after confirmation. Move the full folder to its permanent location first. Open a new terminal afterward.

```powershell
chatgpt-update check
chatgpt-update update
chatgpt-update update --no-backup
chatgpt-update update --backup
chatgpt-update update --exact
chatgpt-update update --plain
chatgpt-update backup
chatgpt-update restore
chatgpt-update self-update
chatgpt-update self-update --yes
chatgpt-update path add
chatgpt-update path remove
chatgpt-update preview
chatgpt-update --version
chatgpt-update --verify-package
```

`check` never installs. `preview` is a simulated offline progress display. `--verify-package` only checks the local scripts against this executable's embedded hashes.

## Backup and restore

Backups cover **`%USERPROFILE%\.codex` only**, not project folders, all app data, or the installed app version. They can contain credentials: keep them private. No backups are uploaded. Custom `CODEX_HOME` locations are not managed by this release.

Keep desktop and CLI/IDE sessions closed while copying state. A manual backup is verified using file SHA-256 hashes. Restore verifies/stages first, requires typing `RESTORE`, preserves the current state in a before-restore backup, and attempts rollback on a failed swap. Legacy `.codex` folder backups are supported with a warning when a historical checksum manifest is unavailable. Links/junctions are not followed.

Manual and engine-created backups are under `%USERPROFILE%\CodexBackups`. The unchanged engine keeps its cache below `%TEMP%\ChatGPT-CorpUpdater-*`. Self-update logs are under `%LOCALAPPDATA%\ChatGPTUpdater\Logs`.

## Updater self-update

Option 4 reads **stable** releases of `Militskiy/chatgpt-update`. It requires the complete portable ZIP, verifies its release checksum, exact file allowlist, component hashes, VERSION and Windows x64 executable format, then hands the operation to the already-installed visible helper. It does not download a script from `main` and execute it.

After the main process exits, the helper verifies the staged package, snapshots old package files, replaces components with the EXE last, and verifies the installed package. Previous files remain in `.chatgpt-update-previous-*` inside the portable folder. An unsuccessful swap attempts component-by-component rollback. Reopen `chatgpt-update` after the helper completes. If execution policy blocks the helper, no replacement occurs.

v0.1.0 cannot automatically migrate to this folder format; do not run the flagged EXE to attempt migration. Extract the complete new package to a clean folder instead. Review prereleases are not selected by self-update and are not evidence of security approval.

## Build, tests and release gating

Only the Go standard library is used. Build on Windows with a supported Go SDK:

```powershell
.\build.ps1
.\tools\Test-Defender.ps1
# Only after the scan succeeds:
go vet ./...
go test -v ./...
.\tests\Test-Scripts.ps1
```

Run the script tests under both Windows PowerShell 5.1 and PowerShell 7. `build.ps1` regenerates `script-hashes.json` from the exact script bytes that it packages, so line endings and approved code-signing changes are accounted for. Do not modify individual scripts after building/distributing the EXE.

CI scans the entire portable folder and final ZIP with updated Defender security intelligence **before launching the built EXE**. A detection, missing scanner, failed signature update, ambiguous result, or changed/deleted artifact blocks distribution. Reports retain hashes, scanner/signature versions, raw results, and cloud/real-time coverage limitations. Tests do not install ChatGPT or change real user PATH/backups.

During investigation the workflow defaults to **review prereleases**. Normal-channel publishing requires a matching version-scoped maintainer approval in `release-approval.json`; the 0.2.1 exception exists solely to exercise v0.2.0's stable-only self-update client. No matching standalone EXE is published, preventing old updaters from silently replacing themselves with an incomplete folder-based package. Release metadata and candidate artifacts are not vendor malware-analysis clearance. Unchanged version tags/assets are never overwritten.
