# Spice Project Model

The Spice Project Model is the single immutable description of a Spice build.
It combines statically decoded project configuration with Go packages, types,
annotations, tests, resources, dependencies, targets, generated source, and
Spice View assignments. The CLI, compiler, LSP, editors, workspace projection,
agents, scaffolds, dependency synchronization, and architecture verification
must consume this same model.

Core owns the public declaration and wire contracts in
`github.com/spice-framework/spice/project`. The separately versioned Toolchain
owns discovery, static decoding, package loading, View inference, dependency
resolution, caching, projection, commands, and diagnostics.

## Configuration files

A project has two required human-authored files:

```text
settings.spice.go
build.spice.go
```

Large builds may add `catalog.spice.go`. A developer may add a gitignored
`local.spice.go` for non-secret machine-local paths and module replacements.
There is no layout selector and no human-authored View, namespace, dependency,
or Project Model JSON file.

Each configuration source starts with the reserved build constraint:

```go
//go:build spice_config

package spice
```

The build tag excludes configuration from ordinary application builds. Spice
also rejects application commands that deliberately activate the tag.

### `settings.spice.go`

Settings own build-wide identity and policy:

```go
//go:build spice_config

package spice

import "github.com/spice-framework/spice/project"

var Settings = project.Settings{
    Name:   "commerce",
    Module: "github.com/acme/commerce",
    Toolchain: project.Toolchain{
        Go:    "1.26.6",
        Spice: "v0.5.0",
    },
    DependencyPolicy: project.DependencyPolicy{
        Verification: project.Strict,
    },
}
```

It owns the project name, canonical Go module, Go and Spice versions, included
projects, approved registries/proxies, and organization-wide dependency
policy. It does not select a filesystem layout.

### `build.spice.go`

Build configuration owns artifact and dependency intent:

```go
//go:build spice_config

package spice

import "github.com/spice-framework/spice/project"

var Build = project.Build{
    Kind: project.Application,
    Dependencies: project.Dependencies{
        project.Starter("web"),
        project.Starter("postgres"),
        project.Library("github.com/google/uuid", "v1.6.0"),
        project.TestLibrary("github.com/stretchr/testify", "v1.11.1"),
    },
    Plugins: project.Plugins{
        project.ApplicationPlugin(),
    },
}
```

It owns application/library/starter/tool kind, dependencies, plugins, targets,
generators, style exceptions, rare View overrides, and packaging. Exceptional
View placement uses canonical Go identity:

```go
Views: project.ViewOverrides{
    project.PlaceType(
        "github.com/acme/commerce/internal/users.UserMapper",
        "users/application",
    ),
},
```

Most files require no override.

### `catalog.spice.go`

An optional catalog carries shared aliases and versions:

```go
//go:build spice_config

package spice

import "github.com/spice-framework/spice/project"

var Catalog = project.Catalog{
    Versions: project.Versions{
        "testify": "v1.11.1",
        "uuid":    "v1.6.0",
    },
}
```

Small applications should not generate a catalog.

### `local.spice.go`

The optional gitignored local file is limited to non-secret local state:

```go
//go:build spice_config

package spice

import "github.com/spice-framework/spice/project"

var Local = project.Local{
    Replacements: project.Replacements{
        project.Replace("github.com/spice-framework/spice", "../spice"),
        project.Replace("github.com/spice-framework/toolchain", "../toolchain"),
    },
    WorkspaceProvider: project.MaterializedWorkspace,
}
```

It may own module replacements, local tool paths, local checkout locations,
and workspace-provider selection. It must never contain credentials or other
secrets.

## Static decoding boundary

Spice parses project configuration without executing it. The closed grammar
accepts:

- string, number, and Boolean literals;
- composite literals;
- known enum selectors;
- lists and maps;
- references to declarations in `catalog.spice.go`;
- an allowlist of recognized `project` constructor calls.

It rejects `init`, loops, arbitrary functions, reflection, environment access,
filesystem reads, process execution, network calls, and immediately invoked
functions. For example, `os.Getenv`, `os.ReadFile`, `exec.Command`,
`http.Get`, and `func() any { ... }()` are never evaluated.

The public constructor bodies exist so the files remain ordinary valid Go and
editor tooling can understand their values. The static decoder recognizes the
call shapes; it does not run those bodies.

## Dependency synchronization

The relationship is:

```text
settings.spice.go + build.spice.go
                 -> spice sync
                 -> go.mod + go.sum
```

Project files express intent. `go.mod` remains the canonical Go module graph,
and `go.sum` authenticates downloaded content. Spice does not introduce a
second dependency resolver or `spice.lock` while selected artifacts are Go
modules.

`spice sync --check` is offline and read-only. `spice sync --adopt` previews
the configuration edits needed to adopt compatible manual Go module changes.

## Published module metadata

Every published Spice-native application, library, starter, and plugin emits a
committed `spice.module.json`. Schema 1 contains:

- artifact kind, name, and Go module;
- minimum and optionally current Spice compatibility;
- capabilities and starter identities;
- annotation and public packages;
- configuration prefixes and compiler tools;
- documentation;
- generated-code requirements.

`project.NewModuleMetadata`, `project.ParseModuleMetadata`, and
`ModuleMetadata.JSON` validate, strictly decode, normalize, and serialize this
contract. Arrays are sorted, duplicate-free, and byte-deterministic. Unknown
JSON fields fail. Published package paths must belong to the declared Go
module. Documentation references are project-relative paths or HTTPS URLs.

The file travels in the ordinary Go module zip and is covered by normal module
content authentication. Metadata never activates a dependency merely because
it is present.

The detailed `spice.starter/v1` manifest remains the annotation/composition
contract during migration. `spice.module.json` is the uniform ecosystem
discovery envelope.

## Wire models

The complete schema identity is:

```text
spice.project-model/v1alpha1
```

A file record contains stable ID, project-relative canonical path, View path,
Go package identity, source set, role, primary symbol, generated/read-only
state, and lowercase SHA-256 content hash. Absolute paths, timestamps, host
details, random values, and environment values are forbidden.

The model validates:

- every path is canonical and project-relative;
- canonical and View paths are unique after case folding;
- every file's package and source set exist;
- generated files are read-only and in the generated source set;
- dependencies have exact resolved Go modules and versions;
- targets refer to known Go packages;
- all output collections have canonical ordering.

The agent schema is:

```text
spice.project-model.agent/v1alpha1
```

Its file type has no canonical-path field. It preserves View path, Go package
identity, source set, role, generated/read-only state, and hash. The default
agent command is:

```text
spice project model --agent --format=json
```

`.spice/cache/project-model.json` is a deterministic, ignored, replaceable
cache. Clients should use `spice project model --format=json` rather than read
it directly.

## Implementation sequence

The ecosystem delivers this feature in bounded, green phases:

1. Freeze public contracts, schema identities, ADRs, and compatibility policy.
2. Implement static project configuration decoding and deterministic model
   construction in Toolchain; migrate Petclinic as the first consumer.
3. Reconcile dependency intent with Go modules and generate module metadata.
4. Deliver read-only View inference, CLI tree output, and GoLand Project View.
5. Add the materialized writable workspace, daemon, journal, Go/Git broker,
   conflict recovery, and `spice shell`.
6. Add View-aware create/move/rename, imports, completion, and architecture
   edges.
7. Integrate Spice Agent workspace descriptors, resolvers, TUI status, and a
   View-safe LSP proxy.
8. Complete GoLand, create the VS Code Explorer repository, and extend Zed.
9. Migrate Commerce, starters, Agent modules, workflows, development catalog,
   and documentation.
10. Add optional ProjFS and FUSE providers only after the portable provider is
    proven.

The first vertical proof spans core contracts, Toolchain, Petclinic, and
GoLand: configuration to Project Model, read-only View tree, materialized
shell, `go test`, `git diff`, and a native breakpoint. Mass migration and
native filesystem providers wait for that path to pass.

## Acceptance

Every implementation phase retains these gates:

- physical `go build ./...`, `go test ./...`, and `go vet ./...`;
- byte-identical Project Model and module metadata;
- reversible canonical/View mapping without case-fold collisions;
- static decoding without application execution or network access;
- source/test/resource/generated classification;
- exact generated read-only enforcement;
- View-path diagnostics with physical source accuracy;
- no silent conflict overwrite;
- no claim that projected visibility is a security sandbox.
