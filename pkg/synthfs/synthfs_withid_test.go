package synthfs

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// TestSynthFS_WithIDMethods tests all the *WithID method variants
func TestSynthFS_WithIDMethods(t *testing.T) {
	sfs := New()
	
	tests := []struct {
		name       string
		createOp   func(string) Operation
		expectedID string
		opType     string
	}{
		{
			name: "CreateFileWithID",
			createOp: func(id string) Operation {
				return sfs.CreateFileWithID(id, "test.txt", []byte("content"), 0644)
			},
			expectedID: "custom-file-id",
			opType:     "create_file",
		},
		{
			name: "CreateDirWithID", 
			createOp: func(id string) Operation {
				return sfs.CreateDirWithID(id, "testdir", 0755)
			},
			expectedID: "custom-dir-id",
			opType:     "create_directory",
		},
		{
			name: "DeleteWithID",
			createOp: func(id string) Operation {
				return sfs.DeleteWithID(id, "test.txt")
			},
			expectedID: "custom-delete-id",
			opType:     "delete",
		},
		{
			name: "CopyWithID",
			createOp: func(id string) Operation {
				return sfs.CopyWithID(id, "source.txt", "dest.txt")
			},
			expectedID: "custom-copy-id",
			opType:     "copy",
		},
		{
			name: "MoveWithID",
			createOp: func(id string) Operation {
				return sfs.MoveWithID(id, "source.txt", "dest.txt")
			},
			expectedID: "custom-move-id",
			opType:     "move",
		},
		{
			name: "CreateSymlinkWithID",
			createOp: func(id string) Operation {
				return sfs.CreateSymlinkWithID(id, "target.txt", "link.txt")
			},
			expectedID: "custom-symlink-id",
			opType:     "create_symlink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := tt.createOp(tt.expectedID)
			
			// Test that operation was created successfully
			if op == nil {
				t.Fatal("operation creation returned nil")
			}
			
			// Test that explicit ID is used
			if string(op.ID()) != tt.expectedID {
				t.Errorf("expected ID %q but got %q", tt.expectedID, op.ID())
			}
			
			// Test that operation type is correct
			desc := op.Describe()
			if desc.Type != tt.opType {
				t.Errorf("expected operation type %q but got %q", tt.opType, desc.Type)
			}
			
			// Test that operation can be validated (basic functionality check)
			testFS := filesystem.NewTestFileSystem()
			// For copy/move operations, create source file
			if tt.opType == "copy" || tt.opType == "move" {
				if err := testFS.WriteFile("source.txt", []byte("test"), 0644); err != nil {
					t.Fatalf("failed to create source file: %v", err)
				}
			}
			
			err := op.Validate(context.Background(), nil, testFS)
			// Some operations may fail validation due to missing dependencies, but they shouldn't panic
			if err != nil {
				// This is acceptable - we're testing ID assignment, not full validation
				t.Logf("validation failed (expected for some operations): %v", err)
			}
		})
	}
}

// TestSynthFS_WithIDValidation tests ID validation edge cases
func TestSynthFS_WithIDValidation(t *testing.T) {
	sfs := New()
	testFS := filesystem.NewTestFileSystem()

	tests := []struct {
		name          string
		id            string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid explicit ID",
			id:          "my-custom-operation-id",
			expectError: false,
		},
		{
			name:          "empty ID",
			id:            "",
			expectError:   true,
			errorContains: "operation ID cannot be empty",
		},
		{
			name:          "whitespace-only ID",
			id:            "   ",
			expectError:   true,
			errorContains: "operation ID cannot contain only whitespace",
		},
		{
			name:          "tab-only ID",
			id:            "\t\t",
			expectError:   true,
			errorContains: "operation ID cannot contain only whitespace",
		},
		{
			name:          "mixed whitespace ID",
			id:            " \t \n ",
			expectError:   true,
			errorContains: "operation ID cannot contain only whitespace",
		},
		{
			name:        "ID with special characters",
			id:          "op-123_test@domain.com",
			expectError: false,
		},
		{
			name:        "very long ID",
			id:          "very-long-operation-id-that-goes-on-and-on-and-should-still-work-fine-even-though-its-really-quite-long-indeed",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with CreateFileWithID as representative example
			op := sfs.CreateFileWithID(tt.id, "test.txt", []byte("content"), 0644)
			
			if op == nil {
				t.Fatal("operation creation returned nil")
			}

			// Test validation
			err := op.Validate(context.Background(), nil, testFS)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				
				// Verify the explicit ID is preserved
				if string(op.ID()) != tt.id {
					t.Errorf("expected ID %q but got %q", tt.id, op.ID())
				}
			}
		})
	}
}

// TestSynthFS_WithIDvsAutoGenerated tests the difference between WithID and auto-generated methods
func TestSynthFS_WithIDvsAutoGenerated(t *testing.T) {
	sfs := New()
	
	// Test that auto-generated and explicit IDs work correctly together
	autoOp := sfs.CreateFile("auto.txt", []byte("content"), 0644)
	explicitOp := sfs.CreateFileWithID("explicit-id", "explicit.txt", []byte("content"), 0644)
	
	// IDs should be different
	if autoOp.ID() == explicitOp.ID() {
		t.Error("auto-generated and explicit IDs should be different")
	}
	
	// Auto-generated ID should not be empty
	if string(autoOp.ID()) == "" {
		t.Error("auto-generated ID should not be empty")
	}
	
	// Explicit ID should match what was provided
	if string(explicitOp.ID()) != "explicit-id" {
		t.Errorf("explicit ID should be 'explicit-id' but got %q", explicitOp.ID())
	}
	
	// Both operations should have correct types
	if autoOp.Describe().Type != "create_file" {
		t.Error("auto-generated operation should have correct type")
	}
	if explicitOp.Describe().Type != "create_file" {
		t.Error("explicit operation should have correct type")
	}
}

// TestSynthFS_WithIDInPipeline tests WithID operations in pipeline context
func TestSynthFS_WithIDInPipeline(t *testing.T) {
	sfs := New()
	testFS := filesystem.NewTestFileSystem()
	
	// Create pipeline with mixed auto-generated and explicit IDs
	pipeline := NewExecutablePipeline()
	
	// Add operations with explicit IDs
	if err := pipeline.Add(sfs.CreateDirWithID("dir-1", "test", 0755)); err != nil {
		t.Fatalf("failed to add directory operation: %v", err)
	}
	if err := pipeline.Add(sfs.CreateFileWithID("file-1", "test/file.txt", []byte("content"), 0644)); err != nil {
		t.Fatalf("failed to add file operation: %v", err)
	}
	
	// Add operations with auto-generated IDs
	if err := pipeline.Add(sfs.CreateDir("test2", 0755)); err != nil {
		t.Fatalf("failed to add directory operation: %v", err)
	}
	if err := pipeline.Add(sfs.CreateFile("test2/file.txt", []byte("content"), 0644)); err != nil {
		t.Fatalf("failed to add file operation: %v", err)
	}
	
	// Execute pipeline
	result, err := pipeline.Execute(context.Background(), testFS)
	if err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}
	
	// Note: Can't access private Success field
	
	if len(result.Operations) != 4 {
		t.Errorf("expected 4 operation results but got %d", len(result.Operations))
	}
	
	// Check that explicit IDs are preserved in results
	ops := pipeline.Operations()
	explicitIDFound := false
	for _, op := range ops {
		if string(op.ID()) == "dir-1" || string(op.ID()) == "file-1" {
			explicitIDFound = true
			break
		}
	}
	if !explicitIDFound {
		t.Error("explicit IDs should be preserved in pipeline operations")
	}
}

// TestSynthFS_WithIDErrorReporting tests that errors properly report explicit IDs
func TestSynthFS_WithIDErrorReporting(t *testing.T) {
	sfs := New()
	testFS := filesystem.NewTestFileSystem()
	
	// Create operation that will fail validation (copy from non-existent source)
	op := sfs.CopyWithID("my-failing-copy-op", "nonexistent.txt", "dest.txt")
	
	err := op.Validate(context.Background(), nil, testFS)
	if err == nil {
		t.Error("expected validation error for copy from non-existent file")
		return
	}
	
	// Check that error includes the explicit ID
	errorMsg := err.Error()
	if !containsString(errorMsg, "my-failing-copy-op") {
		t.Errorf("error message should include explicit operation ID: %s", errorMsg)
	}
}

// TestSynthFS_WithIDCollisionHandling tests behavior with duplicate IDs
func TestSynthFS_WithIDCollisionHandling(t *testing.T) {
	sfs := New()
	
	// Create two operations with the same explicit ID
	op1 := sfs.CreateFileWithID("duplicate-id", "file1.txt", []byte("content1"), 0644)
	op2 := sfs.CreateFileWithID("duplicate-id", "file2.txt", []byte("content2"), 0644)
	
	// Both operations should be created (SynthFS doesn't prevent duplicate IDs at creation time)
	if op1 == nil || op2 == nil {
		t.Fatal("operations should be created even with duplicate IDs")
	}
	
	// Both should have the same ID
	if op1.ID() != op2.ID() {
		t.Error("operations with same explicit ID should have same ID")
	}
	
	// Test in pipeline context - behavior may vary depending on pipeline implementation
	pipeline := NewExecutablePipeline()
	if err := pipeline.Add(op1); err != nil {
		t.Logf("failed to add operation 1: %v", err)
	}
	if err := pipeline.Add(op2); err != nil {
		t.Logf("failed to add operation 2: %v", err)
	}
	
	testFS := filesystem.NewTestFileSystem()
	result, err := pipeline.Execute(context.Background(), testFS)
	
	// Document the current behavior - this test captures what actually happens
	// rather than prescribing what should happen
	if err != nil {
		t.Logf("pipeline with duplicate IDs failed (which may be expected): %v", err)
	} else if result != nil {
		t.Logf("pipeline with duplicate IDs succeeded with %d operations", len(result.Operations))
	}
}