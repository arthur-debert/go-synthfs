package core

import (
	"context"
	"testing"
	"time"
)

func TestOperationEvents(t *testing.T) {
	t.Run("OperationStartedEvent", func(t *testing.T) {
		details := map[string]interface{}{"mode": "0644"}
		event := NewOperationStartedEvent("op1", "create_file", "/test/file.txt", details)

		if event.Type() != EventOperationStarted {
			t.Errorf("Expected event type '%s', got '%s'", EventOperationStarted, event.Type())
		}

		if event.Operation.OperationID != "op1" {
			t.Errorf("Expected operation ID 'op1', got '%s'", event.Operation.OperationID)
		}

		if event.Operation.OperationType != "create_file" {
			t.Errorf("Expected operation type 'create_file', got '%s'", event.Operation.OperationType)
		}

		if event.Operation.Path != "/test/file.txt" {
			t.Errorf("Expected path '/test/file.txt', got '%s'", event.Operation.Path)
		}
	})

	t.Run("OperationCompletedEvent", func(t *testing.T) {
		duration := 100 * time.Millisecond
		event := NewOperationCompletedEvent("op1", "create_file", "/test/file.txt", nil, duration)

		if event.Type() != EventOperationCompleted {
			t.Errorf("Expected event type '%s', got '%s'", EventOperationCompleted, event.Type())
		}

		if event.Duration != duration {
			t.Errorf("Expected duration %v, got %v", duration, event.Duration)
		}
	})

	t.Run("OperationFailedEvent", func(t *testing.T) {
		duration := 50 * time.Millisecond
		testError := context.DeadlineExceeded
		event := NewOperationFailedEvent("op1", "create_file", "/test/file.txt", nil, testError, duration)

		if event.Type() != EventOperationFailed {
			t.Errorf("Expected event type '%s', got '%s'", EventOperationFailed, event.Type())
		}

		if event.Error != testError {
			t.Errorf("Expected error %v, got %v", testError, event.Error)
		}

		if event.Duration != duration {
			t.Errorf("Expected duration %v, got %v", duration, event.Duration)
		}
	})
}
