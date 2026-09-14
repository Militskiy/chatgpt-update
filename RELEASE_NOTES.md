# First portable prototype

Windows x64 single EXE. Numbered menu, embedded v5.4 ChatGPT updater engine, optional upgrade backups, manual verified backups, restore with preserved pre-restore state, user PATH registration, and updater self-update from this repository.

No MSI, Go, PowerShell 7, WinGet or Microsoft account is needed on the target PC. Windows PowerShell 5.1 and permitted signed AppX deployment are required.

The EXE is unsigned. Use only with your organization's approval. ChatGPT MSIX downloads still use the third-party Wangnov/codex-app-mirror. Backups cover USERPROFILE\.codex only and can contain credentials.

Automated tests do not install ChatGPT; real corporate-PC deployment/proxy behavior remains an on-device check.
