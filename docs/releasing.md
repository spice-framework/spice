# Releasing Spice core

This repository publishes the standard-library-only
`github.com/spice-framework/spice` library module. Core releases contain
deterministic source, module provenance, checksums, and keyless provenance.
CLI binaries and toolchain artifacts are released from
[`spice-framework/toolchain`](https://github.com/spice-framework/toolchain).

## Candidate contract

The exact candidate commit must contain this closed `spice-release.json`
identity:

```json
{
  "schema": 1,
  "profile": "go-module-v1",
  "repository": "spice",
  "module": "github.com/spice-framework/spice",
  "version": "v0.1.0-preview.3"
}
```

It must pass `make verify-release` under Go 1.26.5. That command is an
unconditional alias for the complete core `make verify` contract. The reusable
workflow additionally requires a canonical SemVer tag at the exact checked-out
commit, a clean tree, public `spice-framework` ownership, the canonical module
identity, and ancestry from `origin/main`.

The repository caller is deliberately closed:

- it pins
  `spice-framework/.github/.github/workflows/go-module-release.yml` at commit
  `9555dd71eccea98cfa82ca2bff27cedd9d154e4a`;
- it repeats that revision through the required `workflow_commit` input;
- repository-level permissions are empty;
- the release job grants only `contents:write`, `id-token:write`,
  `attestations:write`, and `artifact-metadata:write`; and
- it forwards no secrets and never uses `secrets: inherit`.

The repository quality gate requires the exact caller and metadata bytes, so a
different workflow, pin, permission, input, secret mapping, job, profile, or
version fails before a candidate can be accepted.

## Keyless artifact contract

The organization workflow builds the renderer from immutable development
commit `6210baa460975be0bfcb12c919cab307da8c3f46`. It produces exactly four
deterministic module artifacts:

- `spice_0.1.0-preview.3_source.tar.gz` containing the tagged committed tree
  below one versioned root;
- `spice_0.1.0-preview.3_sbom.spdx.json` containing the one-module SPDX 2.3
  graph;
- `spice_0.1.0-preview.3_release.json` binding repository, module, version,
  commit, source epoch, Go version, and artifact digests; and
- `checksums.txt` containing canonical SHA-256 entries.

Spice core intentionally has no root `go.sum` or `vendor/modules.txt`. The
central renderer accepts that graphless form only because the catalog selects
no required modules and offline, read-only Go inspection proves that `go.mod`
contains no `require`, `tool`, or `replace` directives and that the selected
graph contains only the unversioned main module. A partial graph pair or any
tracked `vendor/` path fails.

The workflow independently builds
`spice-go-release-verify` from toolchain commit
`0bb834c688ae42865a65deb9b8c00d033d359c9d`. That verifier authenticates the
exact Git source, archive, metadata, SBOM, checksums, module policy, and clean
tag identity, then copies only accepted bytes into a new verifier-owned
directory. Renderer output is never passed directly to signing or publication.

The protected `release-attestation` job uses a short-lived GitHub OIDC identity
to create Sigstore-backed SLSA provenance for those independently verified
artifacts. A separate unprivileged job verifies the portable bundle against
the exact caller repository, source commit and tag ref, GitHub issuer,
organization workflow path, and immutable workflow commit. The bundle is
published as `provenance.sigstore.json` beside the four artifacts.

## Protected authority and immutable tags

No long-lived release key or Actions secret is used or forwarded by the
preview.2-and-later path. Two
secret-free protected environments separate authority:

1. `release-attestation` approves the only job that receives OIDC,
   attestation, and artifact-metadata authority; it has no content-write
   permission.
2. `release-publish` approves the only job that receives `contents:write`; it
   cannot mint an OIDC identity and receives only authenticated artifacts.

Both environments accept only `v*` deployment refs and require the current
repository owner as reviewer. Repository rules separately restrict release-tag
creation and prohibit updates or deletion without bypass. A mistaken tag must
be followed by a new version; it is never moved or reused.

The Ed25519 key retained at
[`security/release/history/v0.1.0-preview.1-ed25519-public.pem`](../security/release/history/v0.1.0-preview.1-ed25519-public.pem)
belongs only to the legacy preview.1 design. Preview.1 did not complete a
GitHub Release, and that key is not part of the preview.2-and-later trust
contract.

The published preview.2 release remains bound to its own immutable historical
authorities: organization workflow
`6a0ba9f430304c33bf897f4e2d3f393926f42eb9`, development renderer
`67b9ca3f20793da881beeea05910042a81ad9877`, and independent toolchain verifier
`83c2a7e41945f8e7ce187f5fb333158c4e6ff223`. Preview.3 does not move or reuse
those authorities; its candidate gate accepts only the current commits above.

## Release ceremony

1. Verify the exact candidate metadata, workflow pin, protected environments,
   deployment policies, and immutable tag rules.
2. Run `make fast`, `make check`, and `make verify-release` in a clean checkout
   and require hosted CI for the exact commit to pass.
3. Rehearse the immutable development renderer and independent toolchain
   verifier against the exact candidate in disposable local storage.
4. Create and push one annotated canonical SemVer tag targeting the accepted
   `main` commit.
5. Approve `release-attestation` only after candidate validation, rendering,
   and independent verification succeed.
6. Approve `release-publish` only after portable provenance authentication
   succeeds.
7. Download every published asset, verify checksums and each artifact against
   the Sigstore bundle and exact workflow identity, and confirm the remote tag
   object and peeled commit remain unchanged.

Committing this candidate does not publish preview.3. The version exists only
after the immutable tag completes this ceremony and the downloaded assets are
independently authenticated.
