package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spice-framework/spice/logging"
)

type safeFailure struct{}

func (safeFailure) Error() string { return "unreviewed-secret" }
func (safeFailure) SafeLogError() logging.ErrorDetails {
	return logging.ErrorDetails{Kind: "dependency", Code: "provider.unavailable", Message: "Provider unavailable"}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type panickingHandler struct{ slog.Handler }

func (panickingHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (panickingHandler) Handle(context.Context, slog.Record) error { panic("secret panic") }

type panickingSafeError struct{}

func (panickingSafeError) Error() string { return "secret raw error" }
func (panickingSafeError) SafeLogError() logging.ErrorDetails {
	panic("secret classification panic")
}

func TestLoggerWritesCanonicalJSONAndText(t *testing.T) {
	t.Parallel()
	for _, format := range []logging.Format{logging.FormatJSON, logging.FormatText} {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			scope := logging.Scope{Module: "example.com/shop/orders", Component: "checkout"}
			logger, err := logging.New(logging.Options{
				Application: "shop", Writer: &output, Scopes: []logging.Scope{scope},
				Configuration: logging.Configuration{Format: format, Level: logging.LevelTrace},
			})
			if err != nil {
				t.Fatal(err)
			}
			scoped, err := logger.WithScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			err = scoped.Emit(t.Context(), logging.Record{
				Timestamp: time.Date(2026, 8, 10, 12, 0, 0, 123, time.FixedZone("test", 3600)),
				Level:     logging.LevelInfo, Event: "order.completed", Message: "Order completed",
				Correlation: logging.Correlation{
					TraceID: "0123456789abcdef0123456789abcdef", SpanID: "0123456789abcdef", TraceFlags: 1,
				},
				Fields: []logging.Field{logging.String("order_state", "complete"), logging.Int64("items", 2)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if format == logging.FormatText {
				text := output.String()
				for _, value := range []string{`schema="spice.log/v1"`, `event="order.completed"`, `module="example.com/shop/orders"`, `items="2"`} {
					if !strings.Contains(text, value) {
						t.Fatalf("text output %q lacks %q", text, value)
					}
				}
				return
			}
			var record struct {
				Schema      string         `json:"schema"`
				Timestamp   time.Time      `json:"timestamp"`
				Severity    string         `json:"severity"`
				Event       string         `json:"event"`
				Message     string         `json:"message"`
				Application string         `json:"application"`
				Module      string         `json:"module"`
				Component   string         `json:"component"`
				Attributes  map[string]any `json:"attributes"`
			}
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("decode %q: %v", output.String(), err)
			}
			if record.Schema != "spice.log/v1" || record.Timestamp.Location() != time.UTC ||
				record.Severity != "INFO" || record.Event != "order.completed" ||
				record.Message != "Order completed" || record.Application != "shop" ||
				record.Module != scope.Module || record.Component != scope.Component ||
				record.Attributes["order_state"] != "complete" || record.Attributes["items"] != float64(2) {
				t.Fatalf("record = %#v", record)
			}
		})
	}
}

func TestControllerAppliesExactStartupAndRuntimeLevels(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	module := logging.Scope{Module: "example.com/shop/orders"}
	component := logging.Scope{Module: module.Module, Component: "checkout"}
	logger, err := logging.New(logging.Options{
		Application: "shop", Writer: &output, Scopes: []logging.Scope{module, component},
		Configuration: logging.Configuration{
			Format: logging.FormatJSON, Level: logging.LevelWarn,
			Levels: []logging.LevelRule{{Scope: module, Level: logging.LevelDebug}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	moduleLogger, err := logger.WithScope(module)
	if err != nil {
		t.Fatal(err)
	}
	componentLogger, err := logger.WithScope(component)
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Info(t.Context(), "root.info", "Root info"); err != nil {
		t.Fatal(err)
	}
	if err := moduleLogger.Debug(t.Context(), "module.debug", "Module debug"); err != nil {
		t.Fatal(err)
	}
	if err := componentLogger.Info(t.Context(), "component.info", "Component info"); err != nil {
		t.Fatal(err)
	}
	if err := logger.Controller().Set(component.ID(), logging.LevelError); err != nil {
		t.Fatal(err)
	}
	if err := componentLogger.Warn(t.Context(), "component.warn", "Component warn"); err != nil {
		t.Fatal(err)
	}
	if err := logger.Controller().Reset(component.ID()); err != nil {
		t.Fatal(err)
	}
	if err := componentLogger.Info(t.Context(), "component.reset", "Component reset"); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, event := range []string{"module.debug", "component.info", "component.reset"} {
		if !strings.Contains(text, event) {
			t.Fatalf("output %q lacks %q", text, event)
		}
	}
	for _, event := range []string{"root.info", "component.warn"} {
		if strings.Contains(text, event) {
			t.Fatalf("output %q contains filtered %q", text, event)
		}
	}
	snapshot := logger.Controller().Snapshot()
	if len(snapshot.Scopes) != 3 || snapshot.Scopes[0].Scope != "root" ||
		snapshot.Scopes[1].Scope != component.ID() || snapshot.Scopes[2].Scope != module.ID() {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	stats := logger.Stats()
	if stats.Attempted != 5 || stats.Emitted != 3 || stats.Filtered != 2 || stats.Failed != 0 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestLoggerRejectsInvalidRecordsAndCountsHandlerFailures(t *testing.T) {
	t.Parallel()
	logger, err := logging.New(logging.Options{
		Application: "test", Writer: failingWriter{},
		Configuration: logging.Configuration{Format: logging.FormatJSON, Level: logging.LevelInfo},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []logging.Record{
		{Level: logging.LevelInfo, Event: "Bad Event", Message: "message"},
		{Level: logging.LevelInfo, Event: "valid.event", Message: "message", Fields: []logging.Field{logging.String("event", "reserved")}},
		{Level: logging.LevelInfo, Event: "valid.event", Message: "message", Fields: []logging.Field{logging.String("same", "a"), logging.String("same", "b")}},
		{Level: logging.LevelInfo, Event: "valid.event", Message: "message", Correlation: logging.Correlation{SpanID: "0123456789abcdef"}},
	}
	for _, record := range tests {
		if err := logger.Emit(t.Context(), record); err == nil {
			t.Fatalf("Emit(%#v) succeeded", record)
		}
	}
	if err := logger.Info(t.Context(), "valid.event", "message"); err == nil {
		t.Fatal("handler failure was not returned")
	}
	if stats := logger.Stats(); stats.Attempted != 5 || stats.Failed != 5 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestLoggerSupportsSlogCompatibilityWithoutGlobals(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger, err := logging.New(logging.Options{
		Application: "compat", Writer: &output,
		Configuration: logging.Configuration{Format: logging.FormatJSON, Level: logging.LevelDebug},
	})
	if err != nil {
		t.Fatal(err)
	}
	logger.Slog().WithGroup("request").InfoContext(t.Context(), "handled", slog.String("route", "/safe"))
	if !strings.Contains(output.String(), `"event":"slog.record"`) ||
		!strings.Contains(output.String(), `"request.route":"/safe"`) {
		t.Fatalf("slog output = %q", output.String())
	}
}

func TestSafeErrorClassificationNeverUsesRawTextByDefault(t *testing.T) {
	t.Parallel()
	if details := logging.ClassifyError(errors.New("raw-secret")); details.Kind != "internal" || details.Message != "" {
		t.Fatalf("ordinary error = %#v", details)
	}
	if details := logging.ClassifyError(context.Canceled); details.Kind != "cancelled" {
		t.Fatalf("cancelled = %#v", details)
	}
	if details := logging.ClassifyError(safeFailure{}); details.Kind != "dependency" ||
		details.Code != "provider.unavailable" || details.Message != "Provider unavailable" {
		t.Fatalf("safe error = %#v", details)
	}
	if details := logging.ClassifyError(panickingSafeError{}); details.Kind != "internal" {
		t.Fatalf("panicking safe error = %#v", details)
	}
}

func TestParseLevelRulesRejectsUnknownAndDuplicateScopes(t *testing.T) {
	t.Parallel()
	scope := logging.Scope{Module: "example.com/app"}
	rules, err := logging.ParseLevelRules(scope.ID()+"=debug", []logging.Scope{scope})
	if err != nil || len(rules) != 1 || rules[0].Level != logging.LevelDebug {
		t.Fatalf("rules = %#v, %v", rules, err)
	}
	for _, value := range []string{"missing=info", scope.ID() + "=info," + scope.ID() + "=warn", scope.ID() + "=verbose"} {
		if _, err := logging.ParseLevelRules(value, []logging.Scope{scope}); err == nil {
			t.Fatalf("ParseLevelRules(%q) succeeded", value)
		}
	}
}

func TestControllerConcurrentUpdatesAndEmits(t *testing.T) {
	t.Parallel()
	scope := logging.Scope{Module: "example.com/app"}
	logger, err := logging.New(logging.Options{
		Application: "race", Handler: slog.DiscardHandler, Scopes: []logging.Scope{scope},
		Configuration: logging.Configuration{Level: logging.LevelInfo},
	})
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := logger.WithScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	operationErrors := make(chan error, 1600)
	for index := range 8 {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			for range 100 {
				if index%2 == 0 {
					if err := logger.Controller().Set(scope.ID(), logging.LevelDebug); err != nil {
						operationErrors <- err
					}
				} else {
					if err := logger.Controller().Reset(scope.ID()); err != nil {
						operationErrors <- err
					}
				}
				if err := scoped.Debug(context.Background(), "race.event", "Race event"); err != nil {
					operationErrors <- err
				}
				logger.Controller().Snapshot()
			}
		}(index)
	}
	group.Wait()
	close(operationErrors)
	for err := range operationErrors {
		t.Error(err)
	}
}

func TestConstructorRejectsConflictingOrMissingDependencies(t *testing.T) {
	t.Parallel()
	if _, err := logging.New(logging.Options{Application: "test", Writer: io.Discard, Handler: slog.DiscardHandler}); err == nil {
		t.Fatal("writer and handler conflict succeeded")
	}
	if _, err := logging.New(logging.Options{Application: " "}); err == nil {
		t.Fatal("invalid application succeeded")
	}
	if _, err := logging.New(logging.Options{
		Application: "test", Handler: slog.DiscardHandler,
		Configuration: logging.Configuration{Format: "yaml", Level: logging.LevelInfo},
	}); err == nil {
		t.Fatal("invalid format succeeded")
	}
	logger, err := logging.New(logging.Options{
		Application: "test", Handler: panickingHandler{Handler: slog.DiscardHandler},
		Configuration: logging.Configuration{Level: logging.LevelInfo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Info(context.Background(), "handler.panic", "Handler panic"); err == nil {
		t.Fatal("handler panic was not recovered")
	}
	if stats := logger.Stats(); stats.Failed != 1 || stats.Emitted != 0 {
		t.Fatalf("stats = %+v", stats)
	}
}
