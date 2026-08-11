# Observability

Spice keeps its logging and observation contracts standard-library-first,
instance-owned, and payload-free. The `logging` package owns validated records,
typed fields, exact scopes, concurrent level control, safe error projection,
and canonical JSON/text handlers over the standard `slog.Handler` boundary.

Handwritten applications can construct the same runtime explicitly:

```go
logger, err := logging.New(logging.Options{
    Application: "commerce",
    Writer: os.Stderr,
    Configuration: logging.Configuration{
        Format: logging.FormatJSON,
        Level:  logging.LevelInfo,
    },
    Scopes: []logging.Scope{{Module: "example.com/commerce"}},
})
if err != nil {
    return err
}
logs, err := observability.NewLoggingObservers(logger)
if err != nil {
    return err
}

application, err := generated.NewApplicationWithOptions(ctx, generated.ApplicationOptions{
    HTTPObservers: []web.HTTPObserver{logs.HTTP},
    Observers:     []lifecycle.Observer{logs.Lifecycle},
})
```

`LoggingObservers` supplies adapters for lifecycle, HTTP, observed methods,
authorization, schedules, async tasks, retry, cache, typed events,
transactions, batch, outbox, migrations, and the bounded test-mail sender.
Records contain compiler-owned identities, status, duration, counters, and
fixed outcomes only. They exclude raw paths, request content, cache keys and
values, event payloads, SQL, mail content, principals, and unreviewed errors.

Generated applications can request these adapters on their application marker:

```go
// @Application
// @observability.Logging
func Application(*Server) {}
```

The compiler records the opt-in as typed bootstrap metadata, selects an
injectable `*logging.Logger`, registers compiler-known scopes, and installs the
adapters required by the active feature set. Mandatory framework metrics run
first, logging second, and application observers after them. Importing the
package alone never activates logging.

The safe API deliberately omits arbitrary values and raw errors. Automatic
failure fields use `logging.ClassifyError`; cancellation and deadlines receive
fixed kinds, ordinary errors become `internal`, and only a reviewed
`logging.SafeError` may expose a bounded code or message. `Logger.Slog()` is an
explicit compatibility boundary for third-party slog callers and does not
weaken the safe Spice field API.

Built-in handlers never open files or connect to a service. Applications own
writers and custom handlers. Exact root/module/component levels can change
through the logger's instance-owned controller; a generated application may
expose the same controller with the loopback-only `loggers` management
endpoint.

## OpenTelemetry starter

The independently versioned
[`github.com/spice-framework/starter-otel`](https://github.com/spice-framework/starter-otel)
module adapts generated route and typed module-event seams to the stable
OpenTelemetry Go trace and metric APIs. Its annotation descriptor
contributes the qualified `@otel.Enable` application annotation. Provide the
application-owned OpenTelemetry inputs as an exact bean and explicitly import
the descriptor before enabling the feature:

```text
go get github.com/spice-framework/starter-otel@latest
```

```go
// @Bean
func OpenTelemetryOptions(
    providers *TelemetryProviders,
) spiceotel.Options {
    return spiceotel.Options{
        TracerProvider: providers.Tracer,
        MeterProvider:  providers.Meter,
    }
}

// @Application
// @otel.Enable
func Application(*Server) {}
```

The compiler activates `spiceotel.NewHTTPObserver` only for that annotation,
validates its exact `web.HTTPObserver` output contract and required reachable
`*http.ServeMux`, and carries the feature and starter provenance into the
generation hash. Generated code constructs the observer through the ordinary
provider graph and installs it before route middleware is created. Invalid
provider inputs, a missing mux capability, or an incompatible observer output
fail before generated files are written. Importing the package, adding its
module dependency, or retaining its compatibility manifest without the annotation does
nothing.

Every request creates a server span named from the method and route template.
Spans include stable route ID, module, method, template, response status, and
panic state. The starter records request count, active requests, duration, and
response body size with the same bounded generated labels.

Typed events expose compiler-owned publisher and subscriber module identities.
`NewEventObserver` turns each synchronous delivery into one internal span plus
delivery count, active-delivery, and duration metrics. Attributes contain only
the event ID, module IDs, subscriber ID/order, and a bounded
`success`/`error`/`panic` outcome; event values and error text are never
recorded.

`NewObserver` composes the HTTP and event adapters when one caller-owned value
should observe both seams:

```go
telemetry, err := spiceotel.NewObserver(options)
if err != nil {
    return err
}
application, err := generated.NewApplicationWithOptions(ctx, generated.ApplicationOptions{
    HTTPObservers:  []web.HTTPObserver{telemetry},
    EventObservers: []event.Observer{telemetry},
})
```

Generated event topics already carry publishing and subscribing module
identity and accept `ApplicationOptions.EventObservers`; no runtime module
registry or payload reflection is involved. `@otel.Enable` continues to
compose the HTTP adapter automatically. Event observation remains an explicit
application option so applications can independently choose its sampling and
export lifecycle.

The starter does not install global OpenTelemetry providers, select an
exporter, read environment variables, or contact a collector. Applications own
provider/exporter construction and shutdown deadlines. The starter repository
owns the canonical [dependency
review](https://github.com/spice-framework/starter-otel/blob/main/docs/dependency-review.md),
[support policy](https://github.com/spice-framework/starter-otel/blob/main/docs/support.md),
compatibility manifest, and verification evidence. This core document remains
the ecosystem composition guide, not a duplicate release contract.

Applications that need custom ordering or conditional observation can omit
`@otel.Enable`, call `spiceotel.NewHTTPObserver` themselves, and pass it through
`NewApplicationWithOptions.HTTPObservers`. This remains the explicit
lower-level escape hatch.
