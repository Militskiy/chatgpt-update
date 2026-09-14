# v0.2.1 - self-update test release

A small visible change for testing the portable updater's 0.2.0 -> 0.2.1 update path:

- The menu now displays: **Tip: 1 updates ChatGPT; 4 updates this utility.**
- Version is now 0.2.1; ChatGPT installation, backup/restore, package validation and self-update code are unchanged.
- Added a menu regression test. Existing Windows PowerShell 5.1/7, replacement/rollback, Go and Defender custom-scan checks remain required.

From your existing, permitted v0.2.0 portable folder, choose **4** or run `chatgpt-update self-update`. Confirm the update, wait for the helper to finish, then restart the app and check `chatgpt-update --version` returns `0.2.1`.

**Release-channel exception:** the maintainer requested a live self-update test after a successful personal-PC test of v0.2.0. This version is published as a normal GitHub release because v0.2.0 only queries `/releases/latest` and rejects prereleases. This is delivery metadata, NOT Microsoft security clearance or corporate deployment approval. The exception applies only to version 0.2.1; subsequent versions default to prerelease without a matching release approval.

**Unsigned.** The original v0.1.0 Wacatac report remains unresolved; do not unblock v0.1.0. Updated-definition custom-scan evidence is attached, including cloud/real-time coverage limitations. Signing credentials have not been configured. Do not disable antivirus, change organizational policy or add exclusions to use this build.

Keep the EXE and its companion scripts together. Updating this utility does not update ChatGPT or modify `.codex` state. Previous portable components are retained for rollback.
