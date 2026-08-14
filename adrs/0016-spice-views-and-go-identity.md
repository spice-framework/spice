# ADR 0016: Spice Views preserve canonical Go identity

## Status

Accepted.

## Context

Spice applications benefit from a concise `src/main/go`, `src/test/go`,
resources, and generated-sources presentation. Go still defines package and
type identity through module import paths and package declarations, and its
compiler, debugger, test runner, and ecosystem depend on that identity.

## Decision

Spice Views are the normal developer presentation of a Spice application:

```text
src/main/go
src/main/resources
src/test/go
src/test/resources
build/generated/spice
```

The physical checkout remains ordinary Go, normally using `cmd/`, `internal/`,
and `internal/spicegen/`. A View path classifies and presents a canonical file;
it does not create a package, type, or namespace identity. For example:

```text
project: commerce
canonical symbol: github.com/acme/commerce/internal/users.User
View path: src/main/go/users/domain/User.go
```

Canonical identity is the Go module plus import path and symbol. View mappings
must be reversible and unique after platform case folding. Generated Views are
read-only. Plain physical `go build ./...`, `go test ./...`, and `go vet ./...`
remain supported after synchronization.

Go packages remain coarse architecture boundaries. Within a feature package,
Views may classify declarations as `domain`, `application`, `persistence`,
`web`, `configuration`, or `events`. Later View-reference rules may enforce
finer architecture without replacing Go package or Modulith boundaries.

Hiding physical layout is user experience, not a security boundary. An
explicit diagnostic command may reveal a canonical path.

## Consequences

- Breakpoints, stack frames, imports, and generated calls retain real Go
  symbols.
- Editors and agents can use concise paths without learning repository layout.
- Projected directory structure cannot be passed naively to package-sensitive
  Go tools; adapters must map through the Project Model.
