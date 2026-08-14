# ADR 0021: Annotation imports remain annotation-only

## Status

Accepted.

## Context

Spice already resolves explicit file-scoped named, aliased, and namespace
annotation bindings to statically decoded descriptor packages. Reusing the
same syntax for ordinary application types would create a second import system
and obscure Go dependency edges.

## Decision

`// @import` resolves compile-time annotation descriptors and the exact typed
operands explicitly supported by an annotation descriptor. It never imports
ordinary application declarations or replaces an ordinary Go import.

Application types continue to use Go imports and Go selectors. Spice tooling
may complete, insert, sort, and alias those imports, navigate them through View
paths, or visually conceal a redundant qualifier where an editor can do so
without changing bytes. Compiler identity and dependency analysis always use
the actual Go import.

The existing file-scoped annotation resolver remains the only annotation
namespace mechanism. No product path falls back to annotation names or a
built-in registry.

## Consequences

- `goimports`, gopls, the compiler, and architecture checks see truthful
  dependencies.
- View ergonomics do not introduce ordinary-type annotation imports.
- Annotation and Go import completion may share UI, but not semantics.
