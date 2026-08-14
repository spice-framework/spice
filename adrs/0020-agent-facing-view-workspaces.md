# ADR 0020: Coding agents receive View workspaces by default

## Status

Accepted.

## Context

Process-based coding agents are most effective with ordinary files and stable
semantic commands, but exposing physical layout makes every agent learn
`cmd/`, `internal/`, generated ownership, and package-specific conventions.
Spice Agent already has workspace identity and rooted coding-tool boundaries.

## Decision

External agents launch through `spice shell -- <agent>` and receive the
projected tree as their working directory. Generated or merged `AGENTS.md`
guidance identifies the View source roots, semantic operations, and read-only
generated sources without documenting physical mapping.

`spice project model --agent --format=json` emits schema
`spice.project-model.agent/v1alpha1`. Its type has no canonical-path field. It
contains View paths, Go package identity, source sets, roles, dependencies,
targets, generated/read-only state, and content hashes. Canonical paths are
available only through an explicit diagnostic escape hatch outside the agent
contract.

Spice Agent run identity incorporates the project and View-model digests.
Coding tools preserve their rooted, traversal-safe and atomic-write properties
by accepting an injected View workspace resolver. Tool results expose View
paths only. Provider adapters do not receive View logic and must not leak
physical paths in model requests.

Agents use stable semantic commands for create, move, rename, dependencies,
tree/model inspection, and verification. A later brokered LSP translates View
URIs to canonical gopls and compiler URIs and maps results back.

## Consequences

- Agents can complete normal changes without reading canonical physical paths.
- Resume safety includes project organization, not merely checkout identity.
- The contract improves usability but does not claim to sandbox a process with
  the user's privileges.
