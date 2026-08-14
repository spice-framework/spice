# Product Principles

## Be broader than a router

Spice is an application platform. Routing is only one capability. The value comes from an integrated, consistent developer experience across configuration, lifecycle, security, data, events, observability, testing, and architecture.

## Prefer capability parity to implementation parity

Spice should cover valuable Spring Boot outcomes but implement them with Go-native mechanisms. Generated wrappers replace subclass proxies. Goroutines replace reactive abstractions where appropriate. Go packages and `internal` boundaries become inputs to module verification.

## Make the easy path excellent

A new developer should receive:

- A clear Spice View structure backed by ordinary synchronized Go source.
- Strong defaults.
- Source-positioned diagnostics.
- Generated code that can be inspected.
- One verification command.
- Examples that actually execute.

## Keep one semantic identity

Go module paths, package import paths, and Go symbols are canonical. Spice
Views, source sets, roles, and editor labels improve presentation and
architecture feedback without adding a language namespace, redefining imports,
or changing debugger identity. Every project-aware adapter consumes one
deterministic Project Model rather than rediscovering the repository.

## Use the developer's real tools

Projected workspaces must work with ordinary shells, editors, file APIs, Go,
Git, and debuggers. Spice may map paths, broker package-sensitive commands, and
offer semantic refactors, but it does not replace shell syntax, the Go
compiler, or the Go module resolver. Physical Go workflows remain the explicit
escape hatch and compatibility gate.

## Keep escape hatches explicit

The framework should enforce standards while allowing reviewed exceptions. Escape hatches must be documented, visible, and analyzable—not accidental bypasses.

## Test the developer experience

Every major feature should include:

- A minimal example.
- A modular reference application.
- Failure-case diagnostics.
- Build and startup benchmarks.
- A first-use workflow test.
