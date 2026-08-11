# Security Policy

Spice is pre-alpha and has no stable release line yet. The configured release
path does not imply that an authenticated preview has been published.

Please avoid publicly disclosing exploitable vulnerabilities. Contact the repository owner privately through GitHub where possible and include reproduction steps, affected versions, and impact.

Security-sensitive framework areas include:

- Annotation and generated-code injection.
- Configuration secret exposure.
- HTTP request binding and validation.
- Authentication and authorization defaults.
- Module boundary bypasses.
- Dependency and starter supply-chain behavior.

No security feature is considered complete without negative tests and documented secure defaults.

## Release authenticity

Core preview.2-and-later source releases use GitHub artifact attestations backed by
Sigstore, not a long-lived repository or organization signing key. Candidate
validation and independently pinned rendering and verification have only read
authority. The secret-free protected `release-attestation` job alone receives
short-lived OIDC, attestation, and artifact-metadata authority; the separately
protected `release-publish` job alone receives content-write authority.

Authenticate every downloaded source artifact against its published portable
bundle, the exact `spice-framework/spice` source commit and tag ref, the GitHub
OIDC issuer, and the immutable organization workflow path and commit documented
in [`docs/releasing.md`](docs/releasing.md). Checksums alone do not establish
provenance. Release tags cannot be updated or deleted.

The public key under `security/release/history/` is retained only to audit the
legacy preview.1 design, which did not produce a completed GitHub Release. It
is not an active preview.2-and-later trust anchor. Report any suspected OIDC or
workflow identity mismatch, tag-rule bypass, workflow-pin drift, artifact
substitution, or checksum/attestation mismatch through the private channel
above.
