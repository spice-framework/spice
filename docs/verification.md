# Verification workflow

The core repository uses one cross-platform Go verifier. It is intentionally
smaller than the toolchain verifier because this module owns public libraries,
not compiler, CLI, LSP, generated application, editor, or release-construction
implementation.

The check phase also validates the canonical [`CODE_STYLE.md`](../CODE_STYLE.md)
contract against the complete official annotation descriptor inventory. The
separately versioned Toolchain executes structural and Spice-aware profile
analysis in consuming applications; core prevents the normative document,
schema, diagnostic namespace, descriptor table, and exact reviewed SHA-256
identity from drifting silently. It also strictly decodes the canonical
schema-two JSON example, validates exact source/build selections, proves every
configured rule has a declared structural or typed implementation owner, and
locks diagnostic ordering through mutation tests.

The supplied policy document is retained as provenance by its SHA-256
`0947169de8263c2d3d8971d18a7f8bff4837b62eb3f4aec39de920fdabba0182`.
The reviewed normative `CODE_STYLE.md` schema-two contract has SHA-256
`9beeec406dba8f9a6c288dd83d2bac60955885c7d5811c37518165cf94673f24`.
Core does not run
`spicestyle`, accept `.spice/style.json`, or own compiler implementation; the
Toolchain must consume the schema-two contract before downstream repositories
can claim the new selection and rule-ownership guarantees are executable.

## Feedback loop

Use focused package tests while editing, then:

```text
make fast      # changed packages plus their reverse import/test-import closure
make check     # version, boundaries, docs, formatting, tidy/vendor, and vet
make lint      # allowlisted golangci-lint plus NilAway
make security  # gosec plus govulncheck
make fuzz      # short parser/decoder/validation fuzz smoke
make test      # one shuffled race-enabled public-package pass plus coverage
make offline   # public packages with -mod=vendor and all network resolution off
make verify    # complete core commit gate
make verify-release # unconditional alias for the same complete gate in the protected release workflow
```

`make fast` is a repository-owned Go command, so PowerShell, Linux, and macOS
execute the same selection logic. It reads staged, unstaged, and untracked
paths from Git, ignores `.tmp`, maps changed package-owned files through
`go list`, and tests the affected packages plus every in-module reverse import,
test-import, and external-test-import consumer. A module-file change or a Go
file with uncertain ownership widens safely to every core package. Documentation
and build-contract edits exercise the quality orchestrator; a clean tree does
the same. The selected tests run once with module-read-only, network-disabled
settings and intentionally omit race and coverage instrumentation for speed.

This narrow command never replaces `make check` or `make verify`. It exists to
catch package-local compile and behavior defects before paying for the broader
repository contracts.

`make test` deliberately combines race testing and coverage in one invocation
across the exact 51 public packages. It enforces at least 85% aggregate
public-source statement coverage without adding the repository-only quality
gate to the denominator.

The bounded fuzz phase executes 100 inputs for the SDK protocol and starter
manifest, configuration JSON decoder, expression parser, and web JSON decoder.
It protects high-risk parser and validation surfaces without duplicating a
broad test pass.

## Module and offline policy

The root module is standard-library-only. Verification runs root and tools
`go mod tidy -diff`, requires the root module graph to contain only
`github.com/spice-framework/spice`, refuses a committed `vendor` directory,
and reproduces the expected empty vendor result. The isolated `tools` module
pins quality binaries without entering the public runtime graph.

The offline phase sets `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, and
`GOTOOLCHAIN=local`, then tests every public package with `-mod=vendor`.
Because core has no third-party dependencies, this is a literal vendor-only
product test even though no vendor directory is necessary.

## Repository boundary checks

The verifier rejects compiler, CLI, LSP, bootstrap, generated-toolchain,
release-builder, benchmark-baseline, and fixture ownership in core. It also
checks the canonical module namespace, the external official annotation-tool
path, public import direction, strict API maturity metadata, and complete Spring
coverage dispositions.

The separately versioned repositories own their specialized gates:

- `toolchain`: compiler, generator, CLI, LSP, dev loop, generated output,
  bootstrap, dogfooding, performance budgets, and release construction;
- `goland` and `zed`: packaged-editor and interaction acceptance;
- `starter-*`: dependency review, real-service, offline, and compatibility
  matrices;
- `petclinic` and `commerce`: complete generated application workflows;
- `development`: exact-revision cross-repository compatibility.

Run `make verify` on the exact tree before a core commit. Release decisions
add the coordinated ecosystem evidence; they do not weaken or bypass this gate.
`make verify-release` deliberately executes the same complete verifier and is
the only candidate-owned command run by the pinned organization keyless
source-release workflow. It runs before any OIDC, attestation, or publication
authority exists. Subsequent rendering and independent verification use
separately pinned organization repositories and treat this checkout as inert
source input.
