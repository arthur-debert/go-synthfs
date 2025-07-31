package synthfs

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestConvenienceMethodsWithEnhancedErrors(t *testing.T) {
	ctx := context.Background()
	
	t.Run("WriteFile with enhanced error", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		// Create a file that will conflict
		err := fs.WriteFile("existing.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		
		// Try to create a directory with the same name (should fail)
		err = MkdirAll(ctx, fs, "existing.txt", 0755)
		if err == nil {
			t.Fatal("Expected error when creating directory with existing file name")
		}
		
		// Check error message format
		errMsg := err.Error()
		if !strings.Contains(errMsg, "failed to create directory") {
			t.Errorf("Error should mention 'failed to create directory', got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "'existing.txt'") {
			t.Errorf("Error should include path, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "(operation: create_directory-") {
			t.Errorf("Error should include operation type and ID, got: %s", errMsg)
		}
	})
	
	t.Run("Error context preservation", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		// Use a path that will trigger validation error
		err := WriteFile(ctx, fs, "", []byte("content"), 0644)
		if err == nil {
			t.Fatal("Expected validation error for empty path")
		}
		
		// The enhanced error should wrap the validation error
		if opErr, ok := err.(*OperationError); ok {
			if opErr.Action != "create file" {
				t.Errorf("Expected action 'create file', got: %s", opErr.Action)
			}
			if !strings.Contains(opErr.Error(), "path cannot be empty") {
				t.Error("Should preserve original validation error message")
			}
		} else {
			t.Errorf("Expected OperationError, got: %T", err)
		}
	})
}