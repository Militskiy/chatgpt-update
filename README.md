# ChatGPT Update — portable Windows app

A small, single-file console app for installing/updating the stable `OpenAI.Codex` Windows desktop package without the Microsoft Store client, creating/restoring `.codex` backups, and updating this utility from this repository's releases.

**Prototype v0.1.0. Windows x64 only.** Not an OpenAI or Microsoft product. Use only where your organization's software policies permit it. This utility does not bypass AppLocker/WDAC/Group Policy, import certificates, disable signature validation, or obtain Store licenses.

## Start

Download **chatgpt-update.exe** from [Releases](../../releases/latest). Keep it in a writable folder, for example `%LOCALAPPDATA%\Programs\ChatGPTUpdater`. Double-click it, or run it from PowerShell:

```powershell
.\chatgpt-update.exe
```

The console menu accepts numbers:

```text
1) Check for ChatGPT update / install
2) Create backup
3) Restore backup
4) Update the updater
5) Add this folder to user PATH
0) Exit
```

Option 1 checks versions and asks Y/N before installing anything. Upgrades ask a separate optional backup question. A fresh install does not create an upgrade backup. The live step/checklist and download bar from updater script v5.4 are retained.

There is no MSI installation. PowerShell 7, Go, Git, WinGet and a Microsoft account are **not needed to run the app**. Windows PowerShell 5.1 is used internally for AppX deployment and Windows-specific operations. The relevant scripts are embedded in the executable, extracted to a random temporary directory when needed, and cleaned up afterward.

## Add the command to PATH

Move the EXE to its permanent writable folder first, then use menu option 5 or:

```powershell
.\chatgpt-update.exe path add
```

This changes **user PATH only**, after confirmation. Close all terminal windows and reopen a terminal from Start; the already-open parent PowerShell cannot inherit a changed environment. Then:

```powershell
chatgpt-update
```

Remove the PATH entry before moving/removing the folder:

```powershell
chatgpt-update path remove
```

Portable mode leaves the EXE where you put it; it does not create a Start-menu shortcut or an Apps & Features entry. Backup/cache/log files are outside the EXE folder as described below.

## Commands

| Command | Behavior |
|---|---|
| `chatgpt-update` | Numbered menu |
| `chatgpt-update check` | Read installed/feed/mirror versions, no installation |
| `chatgpt-update update` | Check and ask to install/update ChatGPT |
| `chatgpt-update update --no-backup` | No upgrade backup question; still asks to install |
| `chatgpt-update update --backup` | Choose upgrade backup; still asks to install |
| `chatgpt-update update --exact` | Only the exact OpenAI-advertised build |
| `chatgpt-update update --plain` | Scrolling output instead of live checklist |
| `chatgpt-update backup` | Create a local, verified `.codex` snapshot |
| `chatgpt-update restore` | Choose a snapshot and type RESTORE to confirm |
| `chatgpt-update self-update` | Check this repository and ask to replace the updater |
| `chatgpt-update self-update --yes` | Update the utility without a Y/N question |
| `chatgpt-update preview` | Simulated progress; no network or app changes |
| `chatgpt-update --version` | Print the **updater's** version, not ChatGPT's |

## Two separate update channels

**ChatGPT/Codex desktop app:** the embedded engine reads OpenAI's `windows-store-update.json` as a version reference and scans **third-party `Wangnov/codex-app-mirror`** stable Windows x64 assets. It may offer a mirrored build above or below the feed, but only with an explicit mismatch warning and never as a downgrade. Missing frameworks are reported, not downloaded from guessed endpoints. Windows checks signed-package deployment with `Add-AppxPackage`. The engine verifies identity, publisher, version, package family and available checksums. A mirror checksum is not independent proof of authenticity.

**This utility:** menu option 4 reads the latest stable release in **`Militskiy/chatgpt-update`**. It verifies asset URL, file size, SHA-256 and PE architecture, then starts a separate Windows helper. The old process exits before replacement. The helper rechecks the hash and `--version`, preserves the existing EXE as a timestamped `.previous` file, performs replacement, verifies it and attempts rollback on failure. Run the utility again after completion. Source commits alone are not downloadable updates; a newer published release and version are required.

Release hashes rely on repository/GitHub integrity; they are not a substitute for code signing. This prototype EXE is **unsigned**. Corporate application control/SmartScreen/antivirus may block it. Ask IT to review/approve it instead of disabling protections.

## Backups and restoration

Scope is **`%USERPROFILE%\.codex` only**. Project repositories, arbitrary working folders, desktop-app LocalState and other app locations are not covered. This is a state restore, **not an app-version rollback**. A newer app's database migrations can limit compatibility with older snapshots. Close all CLI/IDE integrations and finish desktop tasks before either operation; the utility detects ordinary `codex` processes and closes only processes belonging to this user's installed stable desktop package. Other integrations may still write state, so the user's explicit confirmation that tasks are finished matters.

Backups are folders under `%USERPROFILE%\CodexBackups`. New manual backups include a file-size/SHA-256 manifest. The utility refuses symlinks/junctions/reparse points and aborts if files change during copying. Backups from the previous PowerShell script (a timestamp folder containing `.codex`) can be restored, with a warning that no original manifest is available.

Restoration verifies and stages a full copy first. The current `.codex` is preserved in a `*-before-restore` backup, then the staged folder replaces it. It does not merge snapshots or silently delete current state. The before-restore snapshot can itself be selected to undo a restore. Backups may contain credentials; do not publish them, send them to coworkers or add them to this repository. No backup is uploaded by this app. Non-default `CODEX_HOME` is not supported by the manual backup/restore commands in v0.1.0.

## Locations and network

- EXE: wherever you placed it. Self-update requires write access to this folder.
- Backups: `%USERPROFILE%\CodexBackups`.
- App download cache: the existing `%TEMP%\ChatGPT-CorpUpdater-*` folders; matching earlier downloads can be reused.
- Helper scripts: random `%TEMP%\ChatGPTUpdater-script-*` folders, normally removed after use.
- Self-update logs: `%LOCALAPPDATA%\ChatGPTUpdater\Logs`.
- Rollback EXE: next to the portable EXE, with a timestamp and `.previous` extension.

Internet access to OpenAI's feed and GitHub/CDN is needed for update commands. Backup and restore are offline. No GitHub token is required for public releases; API rate limits can apply. The Go self-updater honors `HTTPS_PROXY`/`HTTP_PROXY`; it does not automatically interpret a corporate PAC script or implement integrated proxy authentication. TLS verification is never disabled.

The embedded PowerShell process uses `-NoProfile -ExecutionPolicy Bypass` for that child process only, matching the earlier standalone invocation. It does not modify the persisted execution policy, and enforced Group Policy takes precedence.

## Build, test and release

A supported Go SDK is needed **only for development**. No third-party Go modules are used.

```powershell
go test ./...
.\build.ps1
.\tests\Test-Scripts.ps1
```

The source includes Go tests for backup/restore integrity, safe replacement, legacy backups, version comparison, checksum parsing, URL validation and arguments. Offline PowerShell tests parse the scripts, exercise the prior update engine's selection/progress helpers, and test EXE replacement using copies in a temporary directory. These tests do **not** install ChatGPT or use real user state.

The GitHub Actions workflow builds on Windows, runs tests in both Windows PowerShell 5.1 and PowerShell 7, and publishes `chatgpt-update.exe` plus `SHA256SUMS.txt`. To publish an update, change `VERSION` (e.g. `0.1.1`), update `RELEASE_NOTES.md`, and push reviewed changes to `main`. The workflow never overwrites an already-published version. The public release is what menu option 4 detects.

Manual on-device testing is still needed for corporate proxy behavior, live console rendering, organizational AppX policies and real ChatGPT deployment.

## References

- [Microsoft: Add-AppxPackage](https://learn.microsoft.com/powershell/module/appx/add-appxpackage)
- [Microsoft: execution policies](https://learn.microsoft.com/powershell/module/microsoft.powershell.core/about/about_execution_policies)
- [GitHub: release-asset API and digests](https://docs.github.com/rest/releases/assets)
- [Third-party MSIX mirror](https://github.com/Wangnov/codex-app-mirror)
