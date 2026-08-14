# ADR 0018: Project dependency intent reconciles to Go modules

## Status

Accepted.

## Context

Developers need starter aliases, test/build scopes, catalogs, policy, and
helpful dependency operations. Go already owns module version selection,
checksums, proxies, vendor behavior, replacements, and authenticated module
content.

## Decision

`build.spice.go` expresses human dependency intent. `catalog.spice.go` may
provide shared aliases and versions. `spice sync` deterministically reconciles
that intent to committed `go.mod` and `go.sum`; `spice sync --check` is
read-only, and `spice sync --adopt` previews explicit adoption of compatible
manual Go module changes.

Go modules remain the only resolver and artifact integrity mechanism. Spice
does not add a BOM resolver or `spice.lock` while all selected artifacts are Go
modules.

Every published Spice-native module generates and commits
`spice.module.json`. Schema 1 describes kind, identity, Spice compatibility,
capabilities, starters, annotation packages, configuration prefixes, compiler
tools, public packages, documentation, and generated-code requirements. The
file is carried in the ordinary Go module zip and is therefore authenticated
with the module content. It does not activate code by dependency presence.

The existing detailed `spice.starter/v1` annotation/composition manifest may
remain as a starter-specific compiler contract during migration;
`spice.module.json` is the ecosystem discovery and compatibility envelope.

## Consequences

- Dependency UX improves without creating competing graph semantics.
- Offline check mode can compare committed intent and module files without
  downloading anything.
- Module metadata freshness becomes a repository verification gate.
