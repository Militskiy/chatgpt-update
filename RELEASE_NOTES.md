# v0.2.2 - reliable self-update handoff and result reporting

Fixes the automatic helper launch path that could return to PowerShell while leaving the old updater installed.

- Launches the visible helper with proper new-console input/output, rather than null streams.
- Waits for validated helper readiness and explicitly acknowledges it before the parent exits. A blocked, failed or unresponsive helper is not treated as a successful handoff.
- Keeps the operation lock across parent exit, so reopening early cannot race the replacement.
- Records success/failure in the log and a small diagnostic status file in the portable folder. Next launch reports completion, a failure, or a still-running update.
- Tests now exercise the actual new-console launcher, locked-target parent exit, early reopen, failure reporting and rollback in Windows CI.

## Migrating from 0.2.0/0.2.1

The old app launches its own old helper, so merely publishing this release cannot fix that initial handoff. Extract the **entire** 0.2.2 ZIP into a clean folder once, or use the previously confirmed foreground recovery of a validated staged update. Do not edit companion scripts individually: their hashes are bound to the EXE. After that, future self-updates use the corrected launcher.

The ChatGPT installation/update engine, backup/restore semantics and package checks are unchanged. No security settings are changed. Keep the EXE and scripts together. The status file is diagnostic only; it never authorizes an installation.

**Unsigned.** The v0.1.0 Wacatac alert remains unresolved; do not unblock that release. The attached Defender custom-scan evidence records cloud/real-time coverage limitations. A local no-detection result is not a Microsoft analyst verdict. Normal-channel delivery is version-scoped to this maintainer-requested fix/test.
