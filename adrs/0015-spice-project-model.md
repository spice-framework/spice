# ADR 0015: One canonical Spice Project Model

## Status

Accepted.

## Context

The CLI, compiler, LSP, editors, scaffolds, dependency synchronization,
workspace projection, architecture verification, and coding agents currently
have overlapping reasons to discover project configuration, Go packages,
annotations, generated source, tests, and resources. Independent discovery
would produce inconsistent paths, identities, ordering, and diagnostics.

## Decision

The Toolchain builds one immutable Spice Project Model from statically decoded
project configuration plus the existing single typed Go program. The public
wire contract lives in `github.com/spice-framework/spice/project`; parsing,
package loading, inference, caching, and service implementation remain in
`github.com/spice-framework/toolchain`.

The model records project identity, source sets, Go packages, Views, reversible
file mappings, resolved dependencies, targets, roles, generated/read-only
state, and content hashes. Every collection has deterministic ordering. Paths
are slash-separated and project-relative; absolute checkout paths and host
metadata are forbidden. Case-folded path collisions fail even on a
case-sensitive host.

The complete wire schema is `spice.project-model/v1alpha1`. The agent-safe
projection is `spice.project-model.agent/v1alpha1` and has no canonical-path
field. Editors and shells normally request the model through
`spice project model --format=json`; `.spice/cache/project-model.json` is only
a replaceable ignored cache.

The model is derived data. Human-authored `views.json`, `layout.json`,
`namespace.json`, `dependencies.json`, or Project Model JSON files are not
configuration surfaces.

## Consequences

- All project-aware adapters consume one identity and path-mapping authority.
- Model construction can be tested for byte-identical determinism.
- The cache can be deleted without losing user intent.
- Schema changes require normal compatibility handling across Toolchain,
  editors, workspaces, and agents.
