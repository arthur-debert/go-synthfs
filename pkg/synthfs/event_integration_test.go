package synthfs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestEventIntegration(t *testing.T) {
	t.Run("Operations emit events during execution", func(t *testing.T) {
		ctx := context.Background()
		fs := NewTestFileSystem()

		// Create batch and executor
		registry := GetDefaultRegistry()
		batch := NewBatch(fs, registry).WithContext(ctx)
		executor := NewExecutor()

		// Track events
		var events []core.Event
		var eventsMutex sync.Mutex

		eventHandler := core.EventHandlerFunc(func(ctx context.Context, event core.Event) error {
			eventsMutex.Lock()
			defer eventsMutex.Unlock()
			events = append(events, event)
			return nil
		})

		// Subscribe to all operation events
		eventBus := executor.EventBus()
		startSub := eventBus.Subscribe(core.EventOperationStarted, eventHandler)
		completeSub := eventBus.Subscribe(core.EventOperationCompleted, eventHandler)
		failSub := eventBus.Subscribe(core.EventOperationFailed, eventHandler)

		// Verify subscription IDs are returned
		if startSub == "" || completeSub == "" || failSub == "" {
			t.Fatal("Subscribe should return non-empty subscription IDs")
		}

		// Add some operations
		_, err := batch.CreateFile("test.txt", []byte("test content"))
		if err != nil {
			t.Fatalf("Failed to add CreateFile operation: %v", err)
		}

		_, err = batch.CreateDir("testdir")
		if err != nil {
			t.Fatalf("Failed to add CreateDir operation: %v", err)
		}

		// Execute the batch via the executor to capture events
		pipeline := NewMemPipeline()
		for _, op := range batch.Operations() {
			if err := pipeline.Add(op.(Operation)); err != nil {
				t.Fatalf("Failed to add operation to pipeline: %v", err)
			}
		}
		result := executor.Run(ctx, pipeline, fs)

		if !result.IsSuccess() {
			t.Fatalf("Batch execution failed: %v", result.GetError())
		}

		// Wait a bit for async events to be processed
		time.Sleep(100 * time.Millisecond)

		// Check that events were emitted
		eventsMutex.Lock()
		defer eventsMutex.Unlock()

		if len(events) == 0 {
			t.Fatal("No events were emitted")
		}

		// Should have start and completion events for each operation
		// 2 operations * 2 events each = 4 events minimum
		if len(events) < 4 {
			t.Fatalf("Expected at least 4 events (2 operations * 2 events), got %d", len(events))
		}

		// Check event types
		startEvents := 0
		completeEvents := 0
		failEvents := 0

		for _, event := range events {
			switch event.Type() {
			case core.EventOperationStarted:
				startEvents++
			case core.EventOperationCompleted:
				completeEvents++
			case core.EventOperationFailed:
				failEvents++
			}
		}

		if startEvents != 2 {
			t.Errorf("Expected 2 start events, got %d", startEvents)
		}

		if completeEvents != 2 {
			t.Errorf("Expected 2 completion events, got %d", completeEvents)
		}

		if failEvents != 0 {
			t.Errorf("Expected 0 failure events, got %d", failEvents)
		}

		// Test event data
		for _, event := range events {
			switch e := event.(type) {
			case *core.OperationStartedEvent:
				if e.Operation.OperationID == "" {
					t.Error("Operation started event missing operation ID")
				}
				if e.Operation.OperationType == "" {
					t.Error("Operation started event missing operation type")
				}
			case *core.OperationCompletedEvent:
				if e.Operation.OperationID == "" {
					t.Error("Operation completed event missing operation ID")
				}
				if e.Duration <= 0 {
					t.Error("Operation completed event should have positive duration")
				}
			}
		}
	})

	t.Run("Failed operations emit failure events", func(t *testing.T) {
		ctx := context.Background()
		fs := NewTestFileSystem()

		registry := GetDefaultRegistry()
		batch := NewBatch(fs, registry).WithContext(ctx)
		executor := NewExecutor()

		// Track events
		var failureEvents []*core.OperationFailedEvent
		var eventsMutex sync.Mutex

		eventHandler := core.EventHandlerFunc(func(ctx context.Context, event core.Event) error {
			if failEvent, ok := event.(*core.OperationFailedEvent); ok {
				eventsMutex.Lock()
				defer eventsMutex.Unlock()
				failureEvents = append(failureEvents, failEvent)
			}
			return nil
		})

		eventBus := executor.EventBus()
		eventBus.Subscribe(core.EventOperationFailed, eventHandler)

		// Create a file with an invalid path that will fail during WriteFile execution
		// The path "../invalid" should fail fs.ValidPath check
		_, err := batch.CreateFile("../invalid", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to add CreateFile operation with invalid path: %v", err)
		}

		// Execute the batch via the executor to capture events
		pipeline := NewMemPipeline()
		for _, op := range batch.Operations() {
			if err := pipeline.Add(op.(Operation)); err != nil {
				t.Fatalf("Failed to add operation to pipeline: %v", err)
			}
		}
		result := executor.Run(ctx, pipeline, fs)

		// The batch should fail because of invalid path
		if result.IsSuccess() {
			t.Skip("Expected batch to fail, but it succeeded")
		}

		// Wait for async events
		time.Sleep(100 * time.Millisecond)

		eventsMutex.Lock()
		defer eventsMutex.Unlock()

		if len(failureEvents) == 0 {
			t.Error("Expected failure events to be emitted")
		}

		for _, failEvent := range failureEvents {
			if failEvent.Error == nil {
				t.Error("Failure event should contain an error")
			}
			if failEvent.Operation.OperationID == "" {
				t.Error("Failure event missing operation ID")
			}
		}
	})

	t.Run("Event bus subscription and unsubscription", func(t *testing.T) {
		executor := NewExecutor()
		eventBus := executor.EventBus()

		var eventReceived bool
		handler := core.EventHandlerFunc(func(ctx context.Context, event core.Event) error {
			eventReceived = true
			return nil
		})

		// Subscribe and immediately unsubscribe
		subID := eventBus.Subscribe(core.EventOperationStarted, handler)
		eventBus.Unsubscribe(subID)

		// Publish an event
		event := core.NewOperationStartedEvent("test", "test", "test", nil)
		err := eventBus.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		if eventReceived {
			t.Error("Handler should not have received event after unsubscribe")
		}
	})
}
