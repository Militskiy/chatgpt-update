# v0.2.0 — portable-folder security review candidate

**Prerelease, unsigned, not a Microsoft malware-analysis clearance.** The original v0.1.0 has a reported Wacatac.B!ml detection and is marked do not use pending investigation. Do not restore or unblock it.

This candidate removes automatic extraction of embedded PowerShell scripts and all runtime ExecutionPolicy overrides. The complete portable ZIP contains the EXE plus visible, hash-checked companion scripts. Keep them together. Existing organizational script policies remain authoritative and may require IT-approved signing.

The numbered menu, ChatGPT update/install/checklist engine, optional upgrade backups, manual backups, restore, and user PATH commands remain. Self-update now validates and replaces the complete portable package, retains the old components, and tests rollback on failed replacement.

CI requires an updated Defender custom scan of the payload and final ZIP before executing the build, followed by Go tests and Windows PowerShell 5.1/7 smoke tests. `defender-report.json` and raw scanner logs record the exact artifact hashes and scan coverage limitations. A no-detection local scan does not guarantee that corporate/cloud/download protection will allow the package.

This review release is NOT selected automatically by stable self-update. Extract the complete ZIP manually to a clean folder; do not use v0.1.0 to migrate. If protection flags this candidate, stop and submit its exact hash/file for Microsoft/IT review. No security exclusions or bypasses are required or recommended.
