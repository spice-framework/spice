package observability

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spice-framework/spice/async"
	"github.com/spice-framework/spice/batch"
	"github.com/spice-framework/spice/cache"
	"github.com/spice-framework/spice/data"
	spiceevent "github.com/spice-framework/spice/event"
	"github.com/spice-framework/spice/event/outbox"
	"github.com/spice-framework/spice/lifecycle"
	"github.com/spice-framework/spice/logging"
	"github.com/spice-framework/spice/mail/mailtest"
	"github.com/spice-framework/spice/migration"
	"github.com/spice-framework/spice/retry"
	"github.com/spice-framework/spice/schedule"
	"github.com/spice-framework/spice/security"
	"github.com/spice-framework/spice/web"
)

func TestLoggingObserversCoverEverySafeObservationSeam(t *testing.T) {
	t.Parallel()
	const module = "example.com/app"
	var output bytes.Buffer
	logger, err := logging.New(logging.Options{
		Application: "test", Writer: &output,
		Scopes:        []logging.Scope{{Module: module}, {Module: module, Component: "service"}},
		Configuration: logging.Configuration{Format: logging.FormatJSON, Level: logging.LevelTrace},
	})
	if err != nil {
		t.Fatal(err)
	}
	observers, err := NewLoggingObservers(logger)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	secret := errors.New("secret-error-canary")
	observers.Lifecycle(ctx, lifecycle.Observation{
		Module: module, Component: "service", Operation: lifecycle.OperationStart, Phase: lifecycle.PhaseEnd, Err: secret,
	})
	_, finishHTTP := observers.HTTP.BeginHTTP(ctx, web.RouteMetadata{
		ID: "route", Module: module, Method: http.MethodGet, Pattern: "/items/{id}",
	})
	finishHTTP(web.HTTPResult{Status: http.StatusOK, Bytes: 10, Duration: time.Millisecond})
	_, finishMethod := observers.Method.BeginMethod(ctx, MethodDefinition{ID: "method", Module: module, Service: "Service", Method: "Run"})
	finishMethod(MethodResult{Definition: MethodDefinition{ID: "method", Module: module, Service: "Service", Method: "Run"}, Duration: time.Millisecond})
	observers.Authorization(ctx, security.Decision{
		Definition: security.Definition{ID: "policy", Module: module}, Allowed: false,
		Reason: security.ReasonUnauthenticated, Duration: time.Millisecond,
	})
	observers.Schedule(ctx, schedule.Result{Definition: schedule.Definition{ID: "job", Module: module}, Run: 2, Duration: time.Millisecond})
	observers.Async(ctx, async.Result{Definition: async.Definition{ID: "task", Module: module}, Duration: time.Millisecond})
	observers.Retry(ctx, retry.Observation{
		ID: "retry", Module: module, Attempt: retry.Attempt{Number: 1, Max: 2},
		Duration: time.Millisecond, Err: secret, NextBackoff: time.Second,
	})
	observers.Cache(ctx, cache.Observation{
		Definition: cache.Definition{ID: "cache", Module: module}, Operation: cache.OperationGet,
		Duration: time.Millisecond, Hit: true, Size: 1,
	})
	interaction := spiceevent.Interaction{
		Event:      spiceevent.Definition{ID: "event", Module: module},
		Subscriber: spiceevent.SubscriberMetadata{ID: "subscriber", Module: module, Order: 1},
	}
	_, finishEvent := observers.Event.BeginEvent(ctx, interaction)
	finishEvent(spiceevent.Result{Interaction: interaction, Duration: time.Millisecond})
	definition := data.Definition{ID: "transaction", Module: module, Isolation: sql.LevelSerializable, ReadOnly: true}
	_, finishTransaction := observers.Transaction.BeginTransaction(ctx, definition)
	finishTransaction(data.Result{Definition: definition, Duration: time.Millisecond})
	observers.Batch(ctx, batch.Observation{
		Definition: batch.Definition{ID: "batch", Module: module}, Operation: batch.OperationStep,
		Step: "step", Attempt: 1, Duration: time.Millisecond, Completed: true,
	})
	observers.Outbox(ctx, outbox.Observation{
		Topic: "topic", Module: module, Attempt: 1, Duration: time.Millisecond, Published: true, Completed: true,
	})
	observers.Migration(ctx, migration.Observation{Version: 1, Module: module, Name: "create_table", Duration: time.Millisecond})
	observers.Mail(ctx, mailtest.Observation{Attempt: 1, MessageID: "message", Outcome: mailtest.OutcomeDelivered})

	encoded := output.String()
	for _, event := range []string{
		"application.lifecycle", "http.server.request", "method.invocation", "security.authorization",
		"schedule.job", "async.task", "retry.attempt", "cache.operation", "event.delivery",
		"data.transaction", "batch.operation", "outbox.delivery", "migration.execution", "mail.delivery",
	} {
		if !strings.Contains(encoded, `"event":"`+event+`"`) {
			t.Fatalf("output lacks event %q: %s", event, encoded)
		}
	}
	for _, forbidden := range []string{"secret-error-canary", "error_message"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("output contains forbidden %q: %s", forbidden, encoded)
		}
	}
}

func TestNewLoggingObserversRejectsNilLogger(t *testing.T) {
	t.Parallel()
	if _, err := NewLoggingObservers(nil); err == nil {
		t.Fatal("NewLoggingObservers(nil) succeeded")
	}
}
