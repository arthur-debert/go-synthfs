package core

import (
	"context"
	"testing"
	"time"
)

func TestBaseEvent(t *testing.T) {
	t.Run("BaseEvent creation and methods", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		event := NewBaseEvent("test.event", data)

		if event.Type() != "test.event" {
			t.Errorf("Expected event type 'test.event', got '%s'", event.Type())
		}

		if event.Data() == nil {
			t.Error("Expected event data to be set")
		}

		// Compare data content
		eventData, ok := event.Data().(map[string]string)
		if !ok {
			t.Errorf("Expected event data to be map[string]string, got %T", event.Data())
		} else if eventData["key"] != "value" {
			t.Errorf("Expected event data key 'value', got '%s'", eventData["key"])
		}

		// Timestamp should be recent
		now := time.Now()
		if event.Timestamp().After(now) {
			t.Error("Event timestamp should not be in the future")
		}

		// Should be within the last second
		if now.Sub(event.Timestamp()) > time.Second {
			t.Error("Event timestamp should be recent")
		}
	})
}

func TestMemoryEventBus(t *testing.T) {
	// Create a mock logger for testing
	logger := &mockLogger{}

	t.Run("Subscribe and publish", func(t *testing.T) {
		bus := NewMemoryEventBus(logger)

		// Track handler calls
		var handledEvents []Event
		handler := EventHandlerFunc(func(ctx context.Context, event Event) error {
			handledEvents = append(handledEvents, event)
			return nil
		})

		// Subscribe to events
		subID := bus.Subscribe("test.event", handler)
		if subID == "" {
			t.Fatal("Subscribe should return a non-empty subscription ID")
		}

		// Publish an event
		event := NewBaseEvent("test.event", "test data")
		err := bus.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		// Check that handler was called
		if len(handledEvents) != 1 {
			t.Fatalf("Expected 1 handled event, got %d", len(handledEvents))
		}

		if handledEvents[0].Type() != "test.event" {
			t.Errorf("Expected event type 'test.event', got '%s'", handledEvents[0].Type())
		}
	})

	t.Run("Multiple handlers", func(t *testing.T) {
		bus := NewMemoryEventBus(logger)

		var handler1Called, handler2Called bool

		handler1 := EventHandlerFunc(func(ctx context.Context, event Event) error {
			handler1Called = true
			return nil
		})

		handler2 := EventHandlerFunc(func(ctx context.Context, event Event) error {
			handler2Called = true
			return nil
		})

		bus.Subscribe("test.event", handler1)
		bus.Subscribe("test.event", handler2)

		event := NewBaseEvent("test.event", "test data")
		err := bus.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		if !handler1Called {
			t.Error("Handler 1 was not called")
		}

		if !handler2Called {
			t.Error("Handler 2 was not called")
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		bus := NewMemoryEventBus(logger)

		var handlerCalled bool
		handler := EventHandlerFunc(func(ctx context.Context, event Event) error {
			handlerCalled = true
			return nil
		})

		// Subscribe then unsubscribe
		subID := bus.Subscribe("test.event", handler)
		bus.Unsubscribe(subID)

		event := NewBaseEvent("test.event", "test data")
		err := bus.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		if handlerCalled {
			t.Error("Handler should not have been called after unsubscribe")
		}
	})

	t.Run("Different event types", func(t *testing.T) {
		bus := NewMemoryEventBus(logger)

		var type1Called, type2Called bool

		handler1 := EventHandlerFunc(func(ctx context.Context, event Event) error {
			type1Called = true
			return nil
		})

		handler2 := EventHandlerFunc(func(ctx context.Context, event Event) error {
			type2Called = true
			return nil
		})

		bus.Subscribe("type1", handler1)
		bus.Subscribe("type2", handler2)

		// Publish type1 event
		event1 := NewBaseEvent("type1", "data1")
		err := bus.Publish(context.Background(), event1)
		if err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		if !type1Called {
			t.Error("Type1 handler was not called")
		}

		if type2Called {
			t.Error("Type2 handler should not have been called")
		}
	})

	t.Run("PublishAsync", func(t *testing.T) {
		bus := NewMemoryEventBus(logger)

		handlerCalled := make(chan bool, 1)
		handler := EventHandlerFunc(func(ctx context.Context, event Event) error {
			handlerCalled <- true
			return nil
		})

		bus.Subscribe("test.event", handler)

		event := NewBaseEvent("test.event", "test data")
		bus.PublishAsync(context.Background(), event)

		// Wait for async handling
		select {
		case <-handlerCalled:
			// Success
		case <-time.After(time.Second):
			t.Error("Async handler was not called within timeout")
		}
	})
}

// mockLogger is a simple mock implementation of Logger for testing
type mockLogger struct{}

func (l *mockLogger) Info() LogEvent  { return &mockLogEvent{} }
func (l *mockLogger) Debug() LogEvent { return &mockLogEvent{} }
func (l *mockLogger) Warn() LogEvent  { return &mockLogEvent{} }
func (l *mockLogger) Error() LogEvent { return &mockLogEvent{} }
func (l *mockLogger) Trace() LogEvent { return &mockLogEvent{} }

type mockLogEvent struct{}

func (e *mockLogEvent) Str(key, val string) LogEvent                   { return e }
func (e *mockLogEvent) Int(key string, val int) LogEvent               { return e }
func (e *mockLogEvent) Err(err error) LogEvent                         { return e }
func (e *mockLogEvent) Float64(key string, val float64) LogEvent       { return e }
func (e *mockLogEvent) Bool(key string, val bool) LogEvent             { return e }
func (e *mockLogEvent) Dur(key string, val interface{}) LogEvent       { return e }
func (e *mockLogEvent) Interface(key string, val interface{}) LogEvent { return e }
func (e *mockLogEvent) Msg(msg string)                                 {}
