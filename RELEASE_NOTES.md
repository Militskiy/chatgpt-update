# v0.2.4 - close the update helper automatically after success

- Successful self-updates no longer ask for Enter. The helper explicitly closes its
  own PowerShell host after package verification, logging, status persistence and cleanup.
- Handled errors still show their details/log path and wait for Enter; that input then
  closes the helper instead of leaving a spare PowerShell prompt.
- Pre-script security/parse errors remain visible through the launcher's existing
  `-NoExit` option. Execution policy, download markings and trust settings are unchanged.
- Added regression coverage for the actual interactive new-console handoff: verified
  success exits without input, failed validation leaves its error window open, and
  both PowerShell 5.1 and 7 tests exercise success with `-NoExit` and an open stdin.
- Colors, startup Y/N updates, parent/helper handshake, locks, rollback, ChatGPT
  deployment and optional .codex backups remain unchanged.

## First update from 0.2.3

Run the existing app and approve its startup update offer, or select menu 4. This
transition is run by 0.2.3's old helper, so its final Enter prompt/window can appear
one last time. Wait for verified success, close that helper and relaunch; --version
should return 0.2.4. The next self-update started by 0.2.4 will auto-close on success.

## Security prompt

Still unsigned. This release does not bypass, auto-answer or remove a security
warning. Correctly signed companion scripts and organizational publisher trust are
the route for policy-controlled deployment; signing the EXE alone is insufficient.
No signing identity is configured. Keep security controls enabled. The original
v0.1.0 Wacatac report remains unresolved. Defender custom-scan evidence is attached
with its hosted-runner cloud/real-time limitations, not a Microsoft analyst verdict.
Normal-channel delivery is version-scoped to this maintainer-requested UI fix/test.
