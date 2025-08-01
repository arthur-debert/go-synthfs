package synthfs

import (
	"errors"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestOperationError(t *testing.T) {
	t.Run("Basic error formatting", func(t *testing.T) {
		err := &OperationError{
			Op:     "create_file",
			ID:     "create_file-abc123",
			Path:   "/tmp/test.txt",
			Action: "create file",
			Err:    errors.New("permission denied"),
		}
		
		expected := "failed to create file '/tmp/test.txt': permission denied (operation: create_file-abc123)"
		if err.Error() != expected {
			t.Errorf("Expected error message:\n%s\nGot:\n%s", expected, err.Error())
		}
	})
	
	t.Run("Error with context", func(t *testing.T) {
		err := &OperationError{
			Op:     "create_dir",
			ID:     "create_dir-xyz789",
			Path:   "/tmp/testdir",
			Action: "create directory",
			Err:    errors.New("disk full"),
		}
		err = err.WithContext("mode", "0755").WithContext("parent", "/tmp")
		
		msg := err.Error()
		if !strings.Contains(msg, "disk full") {
			t.Error("Error message should contain underlying error")
		}
		if !strings.Contains(msg, "create_dir-xyz789") {
			t.Error("Error message should contain operation ID")
		}
		// Context order is not guaranteed, so just check both are present
		if !strings.Contains(msg, "mode=0755") || !strings.Contains(msg, "parent=/tmp") {
			t.Error("Error message should contain context")
		}
	})
	
	t.Run("Unwrap", func(t *testing.T) {
		baseErr := errors.New("base error")
		err := &OperationError{
			Op:     "delete",
			ID:     "delete-123",
			Path:   "/tmp/file",
			Action: "delete",
			Err:    baseErr,
		}
		
		if errors.Unwrap(err) != baseErr {
			t.Error("Unwrap should return the base error")
		}
	})
}

func TestPipelineError(t *testing.T) {
	t.Run("Pipeline error formatting", func(t *testing.T) {
		baseErr := &OperationError{
			Op:     "create_file",
			ID:     "create_file-3",
			Path:   "/tmp/file3.txt",
			Action: "create file",
			Err:    errors.New("permission denied"),
		}
		
		pipelineErr := &PipelineError{
			FailedOp:      nil, // Would be the actual operation in real use
			FailedIndex:   3,
			TotalOps:      5,
			Err:           baseErr,
			SuccessfulOps: []core.OperationID{"create_dir-1", "create_file-2"},
		}
		
		msg := pipelineErr.Error()
		
		// Check main error message
		if !strings.Contains(msg, "Pipeline execution failed at operation 3 of 5") {
			t.Error("Error should indicate which operation failed")
		}
		
		// Check it includes the underlying error
		if !strings.Contains(msg, "permission denied") {
			t.Error("Error should include underlying error message")
		}
		
		// Check successful operations are listed
		if !strings.Contains(msg, "Previous successful operations:") {
			t.Error("Error should list successful operations")
		}
		if !strings.Contains(msg, "1. create_dir-1") {
			t.Error("Error should list first successful operation")
		}
		if !strings.Contains(msg, "2. create_file-2") {
			t.Error("Error should list second successful operation")
		}
	})
	
	t.Run("Pipeline error without successful ops", func(t *testing.T) {
		pipelineErr := &PipelineError{
			FailedOp:      nil,
			FailedIndex:   1,
			TotalOps:      3,
			Err:           errors.New("failed immediately"),
			SuccessfulOps: nil,
		}
		
		msg := pipelineErr.Error()
		if strings.Contains(msg, "Previous successful operations:") {
			t.Error("Should not mention successful operations when there are none")
		}
	})
}

func TestRollbackError(t *testing.T) {
	t.Run("Rollback error formatting", func(t *testing.T) {
		originalErr := errors.New("disk full")
		rollbackErr := &RollbackError{
			OriginalErr: originalErr,
			RollbackErrs: map[core.OperationID]error{
				"remove-dir-xyz": errors.New("directory not empty"),
				"remove-file-abc": errors.New("file locked"),
			},
		}
		
		msg := rollbackErr.Error()
		
		// Check original error is included
		if !strings.Contains(msg, "Operation failed: disk full") {
			t.Error("Should include original error")
		}
		
		// Check rollback failures are listed
		if !strings.Contains(msg, "Rollback also failed:") {
			t.Error("Should indicate rollback failed")
		}
		if !strings.Contains(msg, "remove-dir-xyz: directory not empty") {
			t.Error("Should list first rollback error")
		}
		if !strings.Contains(msg, "remove-file-abc: file locked") {
			t.Error("Should list second rollback error")
		}
	})
	
	t.Run("Rollback error unwrap", func(t *testing.T) {
		originalErr := errors.New("original error")
		rollbackErr := &RollbackError{
			OriginalErr:  originalErr,
			RollbackErrs: make(map[core.OperationID]error),
		}
		
		if errors.Unwrap(rollbackErr) != originalErr {
			t.Error("Unwrap should return the original error")
		}
	})
}

func TestWrapOperationError(t *testing.T) {
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	
	t.Run("Wrap nil error", func(t *testing.T) {
		op := CreateFile("/tmp/test.txt", []byte("content"), 0644)
		err := WrapOperationError(op, "create file", nil)
		if err != nil {
			t.Error("Wrapping nil error should return nil")
		}
	})
	
	t.Run("Wrap regular error", func(t *testing.T) {
		op := CreateFile("/tmp/test.txt", []byte("content"), 0644)
		baseErr := errors.New("permission denied")
		err := WrapOperationError(op, "create file", baseErr)
		
		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatal("Should return an OperationError")
		}
		
		if opErr.Op != "create_file" {
			t.Errorf("Expected op type 'create_file', got: %s", opErr.Op)
		}
		if opErr.Path != "/tmp/test.txt" {
			t.Errorf("Expected path '/tmp/test.txt', got: %s", opErr.Path)
		}
		if opErr.Action != "create file" {
			t.Errorf("Expected action 'create file', got: %s", opErr.Action)
		}
		if opErr.Err != baseErr {
			t.Error("Should preserve the base error")
		}
	})
	
	t.Run("Don't double-wrap OperationError", func(t *testing.T) {
		op := CreateFile("/tmp/test.txt", []byte("content"), 0644)
		opErr := &OperationError{
			Op:     "create_file",
			ID:     "create_file-123",
			Path:   "/tmp/test.txt",
			Action: "create file",
			Err:    errors.New("base error"),
		}
		
		wrapped := WrapOperationError(op, "create file", opErr)
		if wrapped != opErr {
			t.Error("Should return the same OperationError without double-wrapping")
		}
	})
}

func TestGetOperationAction(t *testing.T) {
	tests := []struct {
		opType   string
		expected string
	}{
		{"create_file", "create file"},
		{"create_directory", "create directory"},
		{"create_symlink", "create symlink"},
		{"delete", "delete"},
		{"copy", "copy"},
		{"move", "move"},
		{"write_file", "write file"},
		{"mkdir", "create directory"},
		{"unknown_op", "unknown_op"},
	}
	
	for _, tt := range tests {
		t.Run(tt.opType, func(t *testing.T) {
			action := getOperationAction(tt.opType)
			if action != tt.expected {
				t.Errorf("Expected action '%s' for op type '%s', got: %s", 
					tt.expected, tt.opType, action)
			}
		})
	}
}