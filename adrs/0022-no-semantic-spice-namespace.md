# ADR 0022: Spice does not define a semantic namespace system

## Status

Accepted.

## Context

A class-oriented View could tempt the project to invent identities such as
`com.acme.commerce.users.domain.User` beside the real Go symbol. Maintaining
two semantic naming systems would complicate type resolution, refactoring,
debugging, module edges, source maps, and generated code.

## Decision

The canonical type identity is the ordinary Go import path plus symbol, for
example `github.com/acme/commerce/internal/users.User`. A View such as
`src/main/go/users/domain/User.go` is presentation metadata. The project name
is build identity. There is no additional semantic Spice namespace.

Source sets, features, roles, View groups, and display filenames may be used
for navigation and architecture policy, but none changes package clauses,
imports, method sets, type identity, generated calls, breakpoints, or stack
frames.

## Consequences

- Go remains the sole type system and compiler authority.
- Cross-feature moves that change Go packages are explicit semantic refactors.
- Same-package View moves can change presentation metadata without changing
  symbols or physical files.
