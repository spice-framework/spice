# Historical release material

The key under `history/` was the reviewed Ed25519 trust anchor for the legacy
`v0.1.0-preview.1` release design. That tag did not produce a completed GitHub
Release. The file is retained only so the historical workflow and tag can be
audited; it is not an active signing key or trust anchor for preview.2 or later.

Current source releases use the keyless organization workflow and a
Sigstore-backed GitHub artifact-attestation bundle as documented in
[`../../docs/releasing.md`](../../docs/releasing.md).
