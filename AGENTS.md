# Development guardrails

Preserve the stable package identity OpenAI.Codex_2p2nqsd0c76g0 and explicit install consent. Never uninstall an existing app as an update strategy or silently substitute a version. Keep backups optional on upgrades and absent on fresh installs. Do not remove TLS/package signature validation or alter corporate policies.

Never commit tokens, real .codex data, local backups or compiled binaries. Changes to restore must stage/verify first, preserve current state and handle rollback. Do not follow symlinks/reparse points in backups.

Keep Windows PowerShell 5.1 compatibility and UTF-8 BOMs in scripts; retain PowerShell 7 test coverage. Use only Go's standard library unless a dependency is expressly reviewed. Run Go tests and both Windows shell smoke suites. Increment VERSION before a release; never overwrite immutable prior release assets. Document any changed sources or scope limitations.
