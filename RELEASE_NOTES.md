# v0.2.5 - end-to-end update test from v0.2.4

A deliberately small release to exercise the already-installed v0.2.4 updater.

- Adds one visible menu hint: **Self-update: auto-close on success; pause on error.**
- Updates the menu regression test and version. No update-engine or helper logic changed.
- Keeps the v0.2.4 verified-success auto-close and error-only pause, the startup
  Y/N prompt, hash checks, locking, rollback, colors and optional backups intact.

## Test from your existing v0.2.4 folder

1. Run `chatgpt-update` and accept the startup offer for v0.2.5, or choose menu 4.
2. After Y, let the separate helper finish. Because this update is initiated by
   v0.2.4, verified success should close that helper without asking for Enter.
3. Do not reopen the updater while replacement is running. Once the helper closes,
   run `chatgpt-update --version`; it should return `0.2.5`.
4. Open the menu and confirm the new hint. ChatGPT and `.codex` are unchanged.

Errors remain visible; handled failures pause for Enter. Any existing Windows
script-security confirmation is separate and unchanged. This release does not
unblock files, change execution policy, auto-answer warnings, or disable protection.

**Unsigned.** Normal-channel delivery is approved only for this maintainer-requested
self-update test. The original v0.1.0 Wacatac report remains unresolved. Defender
custom-scan evidence is attached with hosted-runner cloud/real-time limitations;
a no-detection custom scan is not a Microsoft analyst verdict.
