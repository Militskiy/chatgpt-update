# v0.2.3 - color UI, animated activity and startup update approval

- Colored, grouped numbered menu; added option 6 for an offline UI preview.
- Animated ASCII activity indicators and elapsed time for updater checks, validation,
  backup/restore and helper readiness. Spinners stop before prompts.
- Updater download progress includes real transferred bytes, one percentage, speed and ETA;
  final 100% is printed only after size/hash verification. No fake installation percentage.
- ChatGPT step panel adds animated transfer-stage markers and elapsed-step labels.
- Interactive menu startup automatically checks this repository for a newer updater.
  Five-second metadata timeout; asks Y/N before download/install. N or unavailable metadata
  still opens the menu. Normal releases only. CLI/preview/verification calls stay independent.
- Added --skip-update-check, --no-color, --no-animation and global --plain; honors NO_COLOR.
- Retained the v0.2.2 PowerShell-child environment correction, real-console handoff,
  readiness acknowledgement, lock continuity, verification and rollback.

## Test using your current 0.2.2

Choose 4 or run `chatgpt-update self-update`; accept 0.2.3. Wait for the helper to say
`[OK] Updater package is now 0.2.3.`, then relaunch. No manual ZIP migration is needed
from 0.2.2. New menu/animation/startup behavior becomes active after this update.
Use menu 6 or `chatgpt-update preview` for a simulated, offline demonstration.

The startup prompt for a newer version will appear when a later eligible release is published.
Startup timeout, decline, current/old releases, invalid metadata, and approved/failing handoff
are covered by offline tests; no test silently installs ChatGPT or touches real user data.

**Unsigned.** Normal-channel publication is scoped to the maintainer's request to test
self-updating and this UI. It is not Microsoft security clearance. The v0.1.0 Wacatac
report remains unresolved. Attached Defender custom-scan evidence records missing
cloud/real-time coverage; keep protections enabled and respect corporate policy.
