# ADR 0014: Spice-native structured logging

## Status

Accepted.

## Context

The original `@observability.Logging` renderer created a generated
`*slog.Logger` option and installed only lifecycle and HTTP adapters. The value
was not an ordinary injectable Spice dependency, per-scope runtime control was
absent, and most existing typed observation seams were not represented.

Spice requires deterministic generated Go, exact typed dependencies,
instance-owned runtime state, standard-library-first core packages, safe
metadata, and no global registries or hidden destinations.

## Decision

Spice owns the public `logging` record, field, scope, level, controller, error,
and handler contracts. `log/slog.Handler` is the interoperability and output
adapter boundary; Spice does not replace the standard handler ecosystem or
mutate `slog` globals.

Canonical handlers write versioned `spice.log/v1` JSON or stable developer
text to a caller-owned writer. Safe records accept closed typed fields and
classify errors without raw text. An explicit compatibility adapter accepts
generic slog records at the caller's security boundary.

`@observability.Logging` selects the logger and observation adapters through
the same typed generated application graph as other dependencies. Exact module
and component scopes replace Java package-prefix logger names. Dynamic levels
are instance-owned and are writable over management only through an explicitly
exposed loopback endpoint.

Files, rotation, network delivery, vendor formats, and OpenTelemetry export
remain separately reviewed adapters or starters.

## Consequences

- Components can constructor-inject one application logger without a service
  locator or global default.
- Framework observers emit bounded compiler-owned metadata across all current
  seams, while logging failures remain diagnostic-only.
- Custom standard handlers remain usable, but generic slog attributes are
  deliberately outside the safe Spice field contract.
- Generated configuration and management wiring grow, but runtime discovery,
  reflection, and third-party core dependencies do not.
