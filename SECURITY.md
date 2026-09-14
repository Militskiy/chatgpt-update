# Security status and release policy

## Reported detection

On 2026-09-14 a user reported Microsoft Defender `Trojan:Win32/Wacatac.B!ml` while downloading v0.1.0, before launching it.

- File: `chatgpt-update.exe`
- Release: `v0.1.0`
- SHA-256: `017a28f61b3d6f6aca8a4ed3f44e3a65e0a6bfae5f2fab7ec3f98d5ef68a7a8a`
- Source: commit `935033e5039aaea10171ae2446b06265c356be87`

The original immutable-by-convention release assets are retained for analysis. Its title/body warn against use. No malware-free declaration or Microsoft analyst verdict exists for it.

## Baseline investigation

GitHub Actions run 34838739834 downloaded that exact hash and scanned it without executing it. The custom scan with Defender engine 1.1.26080.3 and security intelligence 1.459.204.0 reported no threats. However, real-time protection was false on the hosted image and `-ValidateMapsConnection` returned `0x80508015`. This result does NOT reproduce or invalidate the user's download/cloud alert. The runner's antivirus settings were not changed and no exclusions were added.

## v0.2.0 hardening

The folder package removes embedded-script extraction and the command-line execution-policy override. Scripts are visible and hash-bound to the executable. Self-update validates an exact complete package and retains rollback copies. These changes reduce unnecessary opaque behavior but do not establish why Defender flagged v0.1.0. Do not repeatedly rebuild or alter packing merely to seek a different classifier result.

CI runs an updated-definition custom scan over the payload and release ZIP. `-DisableRemediation` is used ONLY for the custom scan: Microsoft's scanner documentation says it ignores exclusions, includes archives and records detections without remediating the evidence. It does not disable real-time protection or change preferences. Positive detections and scan failures stop publication. Reports explicitly state missing cloud/real-time coverage. No unsigned prototype is promoted to stable automatically during this investigation without a documented, version-scoped maintainer approval.

## Resolving a continuing detection

Leave the file blocked; do not disable protection or add an exclusion. Submit the exact EXE/hash and source reference through Microsoft Security Intelligence's developer submission process, or ask organizational Defender for Endpoint administrators to submit it. A human must complete the authenticated submission; this repository/CI has not made that submission or obtained a decision. Record the submission ID, file hash and final determination before calling a detection a confirmed false positive.

References:
- https://www.microsoft.com/en-us/wdsi/filesubmission
- https://learn.microsoft.com/en-us/defender-xdr/developer-faq
- https://learn.microsoft.com/en-us/defender-endpoint/admin-submissions-mde
- https://learn.microsoft.com/en-us/defender-endpoint/command-line-arguments-microsoft-defender-antivirus

## Signing

A trusted signing identity is not available in this project yet. No fake/self-signed certificate is installed to manufacture trust. For enterprise distribution, have IT review and sign the scripts before generating embedded hashes; sign the compiled EXE; package and scan the final signed bytes. Signing identifies the publisher but is not an antivirus clearance.

Never put `.codex` state, backups, tokens, certificates/private keys, or confidential data into this public repository or public scanner submissions.

## v0.2.1 normal-channel self-update test

The maintainer reported a successful personal-PC test of v0.2.0 and requested a minor new version to exercise the installed application's updater. Since v0.2.0 queries only GitHub's latest normal release and rejects prereleases, `release-approval.json` records a **version-scoped delivery-channel exception for 0.2.1**. All scan gates and disclosed coverage limitations remain. This is not Microsoft analyst clearance, confirmation of a false positive, or approval for corporate distribution. Subsequent versions default to prerelease without another matching approval. No signing certificate/service is configured yet.
