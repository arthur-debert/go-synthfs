package synthfs_test

import (
	"errors"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/ops"
)

func TestValidationError(t *testing.T) {
	op := ops.NewCreateFile("test.txt", []byte("content"), 0644)
	cause := errors.New("underlying cause")

	err := &synthfs.ValidationError{
		Operation: op,
		Reason:    "invalid path",
		Cause:     cause,
	}

	t.Run("Error method", func(t *testing.T) {
		errMsg := err.Error()
		if errMsg == "" {
			t.Errorf("Expected non-empty error message")
		}

		// Should contain operation ID and reason
		if !contains(errMsg, "invalid path") {
			t.Errorf("Expected error message to contain reason 'invalid path', got %q", errMsg)
		}
	})

	t.Run("Unwrap method", func(t *testing.T) {
		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Errorf("Expected Unwrap to return %v, got %v", cause, unwrapped)
		}

		// Test error chain
		if !errors.Is(err, cause) {
			t.Errorf("Expected errors.Is(err, cause) to be true")
		}
	})
}

func TestDependencyError(t *testing.T) {
	op := ops.NewCreateFile("test.txt", []byte("content"), 0644)
	dependencies := []synthfs.OperationID{"dep1", "dep2"}
	missing := []synthfs.OperationID{"dep1"}

	err := &synthfs.DependencyError{
		Operation:    op,
		Dependencies: dependencies,
		Missing:      missing,
	}

	t.Run("Error method", func(t *testing.T) {
		errMsg := err.Error()
		if errMsg == "" {
			t.Errorf("Expected non-empty error message")
		}

		// Should contain information about missing dependencies
		if !contains(errMsg, "dep1") {
			t.Errorf("Expected error message to contain missing dependency 'dep1', got %q", errMsg)
		}
	})
}

func TestConflictError(t *testing.T) {
	op := ops.NewCreateFile("test.txt", []byte("content"), 0644)
	conflicts := []synthfs.OperationID{"conflict1", "conflict2"}

	err := &synthfs.ConflictError{
		Operation: op,
		Conflicts: conflicts,
	}

	t.Run("Error method", func(t *testing.T) {
		errMsg := err.Error()
		if errMsg == "" {
			t.Errorf("Expected non-empty error message")
		}

		// Should contain information about conflicts
		if !contains(errMsg, "conflict") {
			t.Errorf("Expected error message to contain 'conflict', got %q", errMsg)
		}
	})
}

func TestOperationConflicts(t *testing.T) {
	t.Run("CreateFile Conflicts method", func(t *testing.T) {
		op := ops.NewCreateFile("test.txt", []byte("content"), 0644)
		conflicts := op.Conflicts()

		// Current implementation returns nil
		if conflicts != nil {
			t.Errorf("Expected CreateFile.Conflicts() to return nil, got %v", conflicts)
		}
	})

	t.Run("CreateDir Conflicts method", func(t *testing.T) {
		op := ops.NewCreateDir("testdir", 0755)
		conflicts := op.Conflicts()

		// Current implementation returns nil
		if conflicts != nil {
			t.Errorf("Expected CreateDir.Conflicts() to return nil, got %v", conflicts)
		}
	})
}

func TestErrorChaining(t *testing.T) {
	// Test that our error types work correctly with Go's error wrapping
	originalErr := errors.New("original error")

	validationErr := &synthfs.ValidationError{
		Operation: ops.NewCreateFile("test.txt", []byte("content"), 0644),
		Reason:    "test validation error",
		Cause:     originalErr,
	}

	// Test errors.Is
	if !errors.Is(validationErr, originalErr) {
		t.Errorf("Expected errors.Is(validationErr, originalErr) to be true")
	}

	// Test errors.As
	var targetErr *synthfs.ValidationError
	if !errors.As(validationErr, &targetErr) {
		t.Errorf("Expected errors.As to find ValidationError")
	}

	if targetErr.Reason != "test validation error" {
		t.Errorf("Expected reason 'test validation error', got %q", targetErr.Reason)
	}
}

func TestErrorTypesWithNilCause(t *testing.T) {
	// Test ValidationError with nil cause
	err := &synthfs.ValidationError{
		Operation: ops.NewCreateFile("test.txt", []byte("content"), 0644),
		Reason:    "no underlying cause",
		Cause:     nil,
	}

	unwrapped := err.Unwrap()
	if unwrapped != nil {
		t.Errorf("Expected Unwrap of ValidationError with nil cause to return nil, got %v", unwrapped)
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Errorf("Expected non-empty error message even with nil cause")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
