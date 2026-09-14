# ChatGPT Update - portable Windows application

**0.2.3: color UI and startup updater checks. Windows x64. Unsigned.** Independent utility, not an OpenAI or Microsoft product. Use only with your organization's approval.

> The original 0.1.0 EXE has a reported Defender `Trojan:Win32/Wacatac.B!ml` detection. Do not unblock it. The cause remains unconfirmed and no Microsoft analyst clearance has been obtained. See SECURITY.md and the release's Defender report. A local scan does not guarantee endpoint/cloud acceptance.

## New in 0.2.3: color and startup checks

The menu is grouped into App, Local Data and Updater sections. Cyan marks current activity,
green completion, yellow a question/warning, and red failure. Text labels remain visible
without colors. ASCII spinners show elapsed time during updater metadata/checksum requests,
package validation, helper readiness and local backup/restore work. Downloads show one real
byte-based percentage, transferred size, measured average speed and ETA. The updater ZIP
only reaches 100% after its size and SHA-256 checks pass. The ChatGPT step panel has an
animated active marker and elapsed-step label when refreshed during transfers; blocking
Windows deployment calls keep the active step highlighted, without an invented overall percentage.

**Opening the menu checks for a newer version of this utility, not ChatGPT.** The startup
metadata request has a five-second total timeout. A newer published normal-channel portable
release prompts **Y/N before any installer download**. N postpones it for this session and
opens the menu; option 4 still checks manually. A network error, timeout or malformed release
opens the menu with a warning. Nothing is installed silently. The same package validation,
operation lock, v0.2.2 helper handshake and rollback still apply after approval.

Only interactive menu launches perform this automatic check. `check`, `update`, `backup`,
`restore`, `preview`, `--help`, `--version` and `--verify-package` do not trigger it.
Redirected input/output and CI do not trigger it either. No scheduler, telemetry, startup
registration or background service is installed.

```powershell
chatgpt-update --skip-update-check  # Open menu without an automatic network request
chatgpt-update --no-color           # Disable colors (also honors NO_COLOR)
chatgpt-update --no-animation       # Keep colors but disable spinner motion
chatgpt-update --plain              # Plain scrolling text; no color or animation
chatgpt-update preview              # Offline simulated demonstration; no app changes
```

Flags work before or after commands. The child updater script also receives presentation
preferences. Unsupported/small terminals and redirected output fall back to readable text.
Menu option **6** opens the offline preview. Spinners stop before asking for input.

### Test the release from 0.2.2

Run your existing `chatgpt-update self-update` or choose 4. Accept 0.2.3, wait for the
helper's completion message, and relaunch. Verify `chatgpt-update --version` prints 0.2.3.
The startup auto-check feature is first available after that upgrade. With 0.2.3 current,
it should report no newer release; it will ask to install when a later eligible release exists.

## Fixing the update loop

The fix is **not in 0.2.1**. To bootstrap from 0.2.0/0.2.1, extract the entire new portable ZIP to a clean folder once. The old executable launches its own old helper; it cannot use the fixed launcher before updating itself. Do not replace only the EXE or edit individual scripts.

From 0.2.2, self-update waits for script-level readiness before the parent exits. The helper uses proper new-console input/output, keeps the operation lock across parent exit, and logs failures as well as success. Starting the helper is not treated as a completed update.

```text
[OK] Helper is ready. This app will exit; replacement is NOT complete yet.
```

Wait for this result in the visible helper window before reopening:

```text
[OK] Updater package is now <new version>.
```

A small `.chatgpt-update-status.json` beside the EXE records completion/failure; the next normal launch displays that result or asks you to wait if the helper is still running. It is diagnostic data only, never an authorization to install. `--version` remains machine-readable.

## Portable folder

Download **chatgpt-update-windows-x64.zip** from the intended release. Extract the **entire** ZIP to a writable permanent folder such as `%LOCALAPPDATA%\Programs\ChatGPTUpdater`.

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

Launch the EXE or run from that folder:

```powershell
.\chatgpt-update.exe
```

No MSI, Go, Git, WinGet, PowerShell 7 or Microsoft account is required to run it. Windows PowerShell 5.1 and permission to deploy signed AppX packages are required. Scripts are visible and checked against SHA-256 hashes embedded in the EXE. Missing/altered companion scripts stop execution. No runtime script extraction, execution-policy override, obfuscator, security exclusion or elevated launch is used.

Existing PowerShell policies and downloaded-file markings remain authoritative. Restricted/AllSigned/RemoteSigned or application-control rules may block this unsigned package. The app stops rather than changing policy or unblocking files. IT-approved script signing must happen before generating embedded hashes/building the EXE; then sign the EXE, package, and scan the final bytes. This release is not signed.

## Menu and commands

```text
1) Check for ChatGPT update / install
2) Create backup
3) Restore backup
4) Update the updater
5) Add this folder to user PATH
6) Preview colors and progress (offline)
0) Exit
```

Option 1 retains the v5.4 engine: installed/feed/mirror versions, explicit Y/N consent, optional upgrade backup, cached MSIX reuse, package identity/publisher checks, and a live checklist. Fresh installation has no upgrade backup. The stable package is `OpenAI.Codex_2p2nqsd0c76g0`; its MSIX still comes from the third-party `Wangnov/codex-app-mirror`. Windows validates deployment with `Add-AppxPackage`; missing dependencies and corporate restrictions are reported, not bypassed.

Option 5 adds this folder to **user PATH**, after confirmation. Move the whole package to its permanent location first, then reopen the terminal. Remove the old PATH entry before relocating/removing that old folder.

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

`check` never installs. `preview` is a simulated offline display. `--verify-package` checks the local scripts against this EXE's embedded hashes.

## Backup and restore

Backups cover **`%USERPROFILE%\.codex` only**, not project folders, every desktop-app data location or the installed app version. Custom CODEX_HOME locations are not managed. Backups can contain credentials; keep them private. Nothing is uploaded.

Keep desktop and CLI/IDE sessions closed while copying state. Manual backups verify copied files by SHA-256. Restore verifies/stages first, requires typing `RESTORE`, preserves current state in a before-restore backup, and attempts rollback on failed replacement. Legacy `.codex` folder backups are supported with a warning when no historical checksum manifest is available. Links/junctions are not followed.

Backups: `%USERPROFILE%\CodexBackups`. ChatGPT installer cache: `%TEMP%\ChatGPT-CorpUpdater-*`. Self-update logs: `%LOCALAPPDATA%\ChatGPTUpdater\Logs`.

## Updater package validation and rollback

Option 4 reads stable releases of `Militskiy/chatgpt-update`. It requires the complete portable ZIP, checks the release checksum, exact file allowlist, component hashes, VERSION and x64 EXE format, then invokes the already-installed, hash-checked helper. No script from `main` is downloaded and executed.

The helper validates staged files before signalling readiness. The parent acknowledges this specific attempt, exits and releases its executable. While holding the operation lock, the helper preserves old components, replaces the EXE last and verifies the result. Old files remain in `.chatgpt-update-previous-*`. A failed replacement attempts component-by-component rollback. Validation failures, missing readiness or an unresponsive helper do not authorize replacement. A blocked script is reported without changing execution policy.

Version 0.1.0 cannot migrate to this folder format; do not run the flagged executable. Review prereleases are excluded from self-update. Normal-channel publishing is delivery metadata, not security approval.

## Build and test

Only Go's standard library is used. On Windows with a supported Go SDK:

```powershell
.\build.ps1
.\tools\Test-Defender.ps1
# Only after the scan succeeds:
go vet ./...
go test -v ./...
.\tests\Test-Scripts.ps1
```

Run the script suite in Windows PowerShell 5.1 and PowerShell 7. Build regenerates `script-hashes.json` from exact packaged bytes. Do not edit scripts after building.

CI scans the full portable folder and final ZIP with updated Defender intelligence before launching the built EXE. Detections, unavailable/failed scanners or modified artifacts prevent publication. Raw reports retain file hashes and cloud/real-time coverage limitations. Windows tests now exercise actual new-console input/output, automatic helper readiness, parent exit with a locked target, early reopen, logged validation failure, package replacement and rollback. Tests use temporary fixtures; they do not install ChatGPT or change real user PATH/backups.

Publication defaults to prerelease. A normal-channel release requires matching version-scoped maintainer approval in `release-approval.json`. Version 0.2.2 is approved for the requested handoff fix/test, not a Microsoft malware clearance. Existing tags and assets are never replaced.
