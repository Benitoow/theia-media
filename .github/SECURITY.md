# Security policy

## The thing to understand first

**Theia has no authentication on its LAN port, and that is a decision, not a
gap.** Anyone who can reach TCP `8383` on your network can browse and stream the
library, change settings, start a scan and install an update.

Theia is a single-household media server for a trusted home network. Adding
accounts, passwords and permissions would change what the project is; the
reasoning is written down in [decision 6](../docs/DECISIONS.md) and in
[the founding spec](../docs/spec-fondatrice.md).

**Do not forward TCP `8383` on your router.** If you need access from outside the
house, use the built-in remote access, which is a different door entirely:

- a separate userspace WireGuard listener, encrypted, on its own UDP port;
- one-time device provisioning, with per-device keys and individual revocation;
- **viewer capabilities only** — a remote device can read the catalogue, images,
  streams and progress, and cannot reach settings, scans, onboarding, updates or
  device management;
- no relay, no rendezvous server, no control plane. The port request goes to your
  own gateway over UPnP IGD or NAT-PMP and nowhere else.

Reports that amount to "the LAN port has no login" will be closed with a link to
this page. Reports that the remote-access boundary can be crossed are exactly
what this policy is for.

## Supported versions

Only the latest published release is supported. Theia updates itself from GitHub
Releases, verifies the SHA-256 digest before installing, and keeps the previous
binary as a rollback target.

| Version | Supported |
|---|---|
| Latest release | Yes |
| Anything older | No — update first |

## Reporting a vulnerability

**Do not open a public issue.**

Use GitHub's private reporting:
[Report a vulnerability](https://github.com/Benitoow/theia-media/security/advisories/new).

Useful reports include what an attacker must already have — network position,
a provisioned device, physical access — and how to reproduce it. A proof of
concept against a local instance is worth more than a description.

This is a single-maintainer hobby project with no bug bounty and no guaranteed
response time. What is guaranteed: a genuine report gets an honest answer, and a
fix ships as a release with the reasoning recorded in the decision log.

## Scope

**In scope**

- Crossing the remote-access boundary: reaching an administrative route from a
  provisioned device.
- Anything that lets a remote peer act without a valid WireGuard handshake.
- Path traversal out of the configured library folders.
- The update path: installing a binary whose digest does not match the published
  one, or defeating the rollback.
- The image and subtitle endpoints being used to read or serve arbitrary files.
- Secrets leaking into logs, the API or the repository — the TMDB key in
  particular.

**Out of scope**

- No authentication on the LAN port. See above.
- Plain HTTP on the LAN. Remote access is encrypted; the local port is not.
- Denial of service by someone who is already on your network.
- Vulnerabilities in FFmpeg itself. Theia pins a checksum-verified build and does
  not patch it; report those upstream.
