# ADR 0017: Declarative `.spice.go` configuration is statically decoded

## Status

Accepted.

## Context

Project configuration should be typed, navigable valid Go without allowing
project discovery to execute arbitrary application or configuration code.
Environment, filesystem, process, and network access during discovery would
make the Project Model unsafe and nondeterministic.

## Decision

A Spice project has two required human-authored files:

- `settings.spice.go` owns project identity, module, toolchain versions,
  included projects, and build-wide dependency policy.
- `build.spice.go` owns artifact kind, dependencies, plugins, targets,
  generators, style exceptions, rare View overrides, and packaging.

`catalog.spice.go` and gitignored `local.spice.go` are optional. Local
configuration is limited to replacements, local tool paths, and workspace
provider selection and must never contain secrets.

Every file uses `//go:build spice_config` and `package spice`. The Toolchain
parses declarations without building or executing them. Its closed expression
grammar accepts literals, composite literals, known enum selectors, lists,
maps, references to catalog declarations, and an allowlist of public
`project` constructor calls. It rejects initialization, loops, arbitrary
functions, reflection, environment reads, file I/O, processes, network calls,
and immediately invoked functions. Application commands that deliberately
activate `spice_config` are rejected.

The public declaration types and recognized constructor shapes live in the
core `project` package. Decoder implementation and diagnostics remain
Toolchain-owned.

No layout selector exists. Views are the standard Spice presentation.

## Consequences

- Configuration remains valid Go and benefits from editor type information.
- Project discovery is offline and does not execute user code.
- Adding expression forms or recognized constructors is a versioned language
  change, not ordinary Go evaluation.
