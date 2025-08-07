package execution_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// TestNewMemPipeline tests pipeline creation
func TestNewMemPipeline(t *testing.T) {
	t.Run("NewMemPipeline with logger", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		if pipeline == nil {
			t.Fatal("Expected pipeline to be created")
		}

		// Should have no operations initially
		ops := pipeline.Operations()
		if len(ops) != 0 {
			t.Errorf("Expected 0 operations initially, got %d", len(ops))
		}
	})

	t.Run("NewMemPipeline with nil logger", func(t *testing.T) {
		pipeline := execution.NewMemPipeline(nil)

		if pipeline == nil {
			t.Fatal("Expected pipeline to be created even with nil logger")
		}

		// Should have no operations initially
		ops := pipeline.Operations()
		if len(ops) != 0 {
			t.Errorf("Expected 0 operations initially, got %d", len(ops))
		}
	})
}

// TestPipeline_Add tests adding operations to the pipeline
func TestPipeline_Add(t *testing.T) {
	t.Run("Add valid operations", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		op2 := NewMockPipelineOperation("op2", "create_directory", "dir1")

		err := pipeline.Add(op1, op2)
		if err != nil {
			t.Fatalf("Expected no error adding operations, got: %v", err)
		}

		ops := pipeline.Operations()
		if len(ops) != 2 {
			t.Errorf("Expected 2 operations, got %d", len(ops))
		}
	})

	t.Run("Add nil operation", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		err := pipeline.Add(nil)
		if err == nil {
			t.Error("Expected error for nil operation")
		}

		expectedErr := "cannot add a nil operation to the pipeline"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("Add invalid operation type", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		err := pipeline.Add("invalid operation")
		if err == nil {
			t.Error("Expected error for invalid operation type")
		}

		expectedErr := "invalid operation type: expected OperationInterface"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("Add duplicate operation IDs", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		op2 := NewMockPipelineOperation("op1", "create_directory", "dir1") // Same ID

		err := pipeline.Add(op1)
		if err != nil {
			t.Fatalf("First add should succeed, got: %v", err)
		}

		err = pipeline.Add(op2)
		if err == nil {
			t.Error("Expected error for duplicate operation ID")
		}

		expectedErr := "operation with ID 'op1' already exists in the pipeline"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("Add multiple operations at once", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		ops := []interface{}{
			NewMockPipelineOperation("op1", "create_file", "file1.txt"),
			NewMockPipelineOperation("op2", "create_directory", "dir1"),
			NewMockPipelineOperation("op3", "create_symlink", "link1"),
		}

		err := pipeline.Add(ops...)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		resultOps := pipeline.Operations()
		if len(resultOps) != 3 {
			t.Errorf("Expected 3 operations, got %d", len(resultOps))
		}
	})
}

// TestPipeline_Operations tests the Operations method
func TestPipeline_Operations(t *testing.T) {
	t.Run("Operations returns copy", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		err := pipeline.Add(op1)
		if err != nil {
			t.Fatalf("Failed to add operation: %v", err)
		}

		ops1 := pipeline.Operations()
		ops2 := pipeline.Operations()

		// Should be different slices (copies)
		if &ops1[0] == &ops2[0] {
			t.Error("Operations should return copies, not the same slice")
		}

		// But should contain the same operations
		if len(ops1) != len(ops2) {
			t.Errorf("Expected same number of operations, got %d vs %d", len(ops1), len(ops2))
		}
	})

	t.Run("Operations preserves order", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		op2 := NewMockPipelineOperation("op2", "create_directory", "dir1")
		op3 := NewMockPipelineOperation("op3", "create_symlink", "link1")

		err := pipeline.Add(op1, op2, op3)
		if err != nil {
			t.Fatalf("Failed to add operations: %v", err)
		}

		ops := pipeline.Operations()
		if len(ops) != 3 {
			t.Fatalf("Expected 3 operations, got %d", len(ops))
		}

		// Check order
		expectedIDs := []string{"op1", "op2", "op3"}
		for i, op := range ops {
			if pipelineOp, ok := op.(execution.OperationInterface); ok {
				if string(pipelineOp.ID()) != expectedIDs[i] {
					t.Errorf("Expected operation %d to have ID %s, got %s", i, expectedIDs[i], pipelineOp.ID())
				}
			}
		}
	})
}

// TestPipeline_Resolve tests dependency resolution
func TestPipeline_Resolve(t *testing.T) {
	t.Run("Resolve empty pipeline", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		err := pipeline.Resolve()
		if err != nil {
			t.Errorf("Expected no error for empty pipeline, got: %v", err)
		}
	})

	t.Run("Resolve already resolved pipeline", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		err := pipeline.Add(op1)
		if err != nil {
			t.Fatalf("Failed to add operation: %v", err)
		}

		// First resolve
		err = pipeline.Resolve()
		if err != nil {
			t.Fatalf("First resolve should succeed, got: %v", err)
		}

		// Second resolve should be no-op
		err = pipeline.Resolve()
		if err != nil {
			t.Errorf("Second resolve should succeed, got: %v", err)
		}
	})

	t.Run("Resolve operations without dependencies", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		op2 := NewMockPipelineOperation("op2", "create_directory", "dir1")
		err := pipeline.Add(op1, op2)
		if err != nil {
			t.Fatalf("Failed to add operations: %v", err)
		}

		err = pipeline.Resolve()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Order should be preserved for independent operations
		ops := pipeline.Operations()
		if len(ops) != 2 {
			t.Errorf("Expected 2 operations, got %d", len(ops))
		}
	})

	// Note: Dependency resolution tests have been removed as dependency tracking
	// was removed from the pipeline implementation



}

// TestPipeline_ResolvePrerequisites tests prerequisite resolution
func TestPipeline_ResolvePrerequisites(t *testing.T) {
	t.Run("ResolvePrerequisites empty pipeline", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()

		err := pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected no error for empty pipeline, got: %v", err)
		}
	})

	t.Run("ResolvePrerequisites with satisfied prerequisites", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()

		// Create operation with prerequisites that are already satisfied
		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		prereq := NewMockPrerequisite("parent_dir", ".", nil) // Current dir always exists
		op1.SetPrerequisites([]core.Prerequisite{prereq})

		err := pipeline.Add(op1)
		if err != nil {
			t.Fatalf("Failed to add operation: %v", err)
		}

		err = pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected no error for satisfied prerequisites, got: %v", err)
		}

		// Should not add new operations
		ops := pipeline.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation (no new ones added), got %d", len(ops))
		}
	})

	t.Run("ResolvePrerequisites with unsatisfied prerequisites", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()

		// Create operation with unsatisfied prerequisite
		op1 := NewMockPipelineOperation("op1", "create_file", "parent/file.txt")
		prereq := NewMockPrerequisite("parent_dir", "parent", errors.New("not satisfied"))
		op1.SetPrerequisites([]core.Prerequisite{prereq})

		err := pipeline.Add(op1)
		if err != nil {
			t.Fatalf("Failed to add operation: %v", err)
		}

		err = pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// The existing MockPrerequisiteResolver returns empty slice, so no new operations added
		ops := pipeline.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation (original only), got %d", len(ops))
		}
	})

	t.Run("ResolvePrerequisites with resolver working normally", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()

		op1 := NewMockPipelineOperation("op1", "create_file", "file.txt")
		prereq := NewMockPrerequisite("parent_dir", "parent", errors.New("not satisfied"))
		op1.SetPrerequisites([]core.Prerequisite{prereq})

		if err := pipeline.Add(op1); err != nil {
			t.Fatal(err)
		}

		err := pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected no error with basic resolver, got: %v", err)
		}

		// Should have 1 operation (original)
		ops := pipeline.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})

	t.Run("ResolvePrerequisites with multiple operations", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()

		// Create two operations with prerequisites
		op1 := NewMockPipelineOperation("op1", "create_file", "parent/file1.txt")
		op2 := NewMockPipelineOperation("op2", "create_file", "parent/file2.txt")

		prereq1 := NewMockPrerequisite("parent_dir", "parent", errors.New("not satisfied"))
		prereq2 := NewMockPrerequisite("parent_dir", "parent", errors.New("not satisfied"))

		op1.SetPrerequisites([]core.Prerequisite{prereq1})
		op2.SetPrerequisites([]core.Prerequisite{prereq2})

		if err := pipeline.Add(op1, op2); err != nil {
			t.Fatal(err)
		}

		err := pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Should have 2 operations (original operations only)
		ops := pipeline.Operations()
		if len(ops) != 2 {
			t.Errorf("Expected 2 operations (original only), got %d", len(ops))
		}
	})

	t.Run("ResolvePrerequisites with unknown prerequisite type", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()

		op1 := NewMockPipelineOperation("op1", "create_file", "file.txt")
		prereq := NewMockPrerequisite("unknown_type", "something", errors.New("not satisfied"))
		op1.SetPrerequisites([]core.Prerequisite{prereq})

		if err := pipeline.Add(op1); err != nil {
			t.Fatal(err)
		}

		err := pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected no error for unknown prerequisite type, got: %v", err)
		}

		// Should have original operation only
		ops := pipeline.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation (original only), got %d", len(ops))
		}
	})
}

// TestPipeline_Validate tests pipeline validation
func TestPipeline_Validate(t *testing.T) {
	t.Run("Validate empty pipeline", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		fs := NewMockFileSystem()
		ctx := context.Background()

		err := pipeline.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no error for empty pipeline, got: %v", err)
		}
	})

	t.Run("Validate successful operations", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		fs := NewMockFileSystem()
		ctx := context.Background()

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		op2 := NewMockPipelineOperation("op2", "create_directory", "dir1")

		if err := pipeline.Add(op1, op2); err != nil {
			t.Fatal(err)
		}

		err := pipeline.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no error for valid operations, got: %v", err)
		}
	})


	t.Run("Validate with operation validation failure", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		fs := NewMockFileSystem()
		ctx := context.Background()

		op1 := NewMockPipelineOperation("op1", "create_file", "file1.txt")
		op1.SetValidateError(errors.New("validation failed"))

		if err := pipeline.Add(op1); err != nil {
			t.Fatal(err)
		}

		err := pipeline.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected error for operation validation failure")
		}

		if !strings.Contains(err.Error(), "validation failed for operation op1") {
			t.Errorf("Expected operation validation error, got: %v", err)
		}
	})


}

// TestPipeline_Integration tests integration scenarios
func TestPipeline_Integration(t *testing.T) {
	t.Run("Full pipeline workflow", func(t *testing.T) {
		logger := NewMockLogger()
		pipeline := execution.NewMemPipeline(logger)
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()
		ctx := context.Background()

		// Set up basic resolver (returns empty operations)

		// Create operations with prerequisites only (dependencies removed)
		op1 := NewMockPipelineOperation("op1", "create_directory", "base")
		op2 := NewMockPipelineOperation("op2", "create_file", "parent/file.txt")
		op3 := NewMockPipelineOperation("op3", "create_file", "base/file.txt")

		// op2 has unsatisfied prerequisite
		prereq := NewMockPrerequisite("parent_dir", "parent", errors.New("not satisfied"))
		op2.SetPrerequisites([]core.Prerequisite{prereq})

		// Add operations
		err := pipeline.Add(op3, op2, op1) // Random order
		if err != nil {
			t.Fatalf("Failed to add operations: %v", err)
		}

		// Resolve prerequisites
		err = pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Fatalf("Failed to resolve prerequisites: %v", err)
		}

		// Resolve (no dependencies to resolve now)
		err = pipeline.Resolve()
		if err != nil {
			t.Fatalf("Failed to resolve: %v", err)
		}

		// Validate
		err = pipeline.Validate(ctx, fs)
		if err != nil {
			t.Fatalf("Failed to validate pipeline: %v", err)
		}

		// Check final state - should have original 3 operations since MockPrerequisiteResolver doesn't add any
		ops := pipeline.Operations()
		if len(ops) != 3 {
			t.Errorf("Expected 3 operations (original), got %d", len(ops))
		}

		// Check that all operations are still present
		foundOrder := make([]string, len(ops))
		for i, op := range ops {
			pipelineOp := op.(execution.OperationInterface)
			foundOrder[i] = string(pipelineOp.ID())
		}

		// Verify all operations exist
		op1Idx := findIndex(foundOrder, "op1")
		op3Idx := findIndex(foundOrder, "op3")
		op2Idx := findIndex(foundOrder, "op2")

		if op1Idx == -1 || op3Idx == -1 || op2Idx == -1 {
			t.Errorf("Missing expected operations in final order: %v", foundOrder)
		}
	})
}

// TestNoOpLogger tests the no-op logger functionality
func TestNoOpLogger(t *testing.T) {
	t.Run("NoOpLogger methods through pipeline operations", func(t *testing.T) {
		// Create a pipeline with nil logger to trigger noOpLogger usage
		pipeline := execution.NewMemPipeline(nil)

		// Verify pipeline was created successfully
		if pipeline == nil {
			t.Fatal("Expected pipeline to be created with nil logger")
		}

		// Test basic operations to ensure noOpLogger works
		op1 := NewMockPipelineOperation("op1", "create_file", "test.txt")
		err := pipeline.Add(op1)
		if err != nil {
			t.Errorf("Expected Add to work with noOpLogger, got error: %v", err)
		}

		// Test other pipeline methods with noOpLogger
		err = pipeline.Resolve()
		if err != nil {
			t.Errorf("Expected Resolve to work with noOpLogger, got error: %v", err)
		}

		// Add another operation for validation test
		op2 := NewMockPipelineOperation("op2", "create_directory", "testdir")
		err = pipeline.Add(op2)
		if err != nil {
			t.Errorf("Expected Add to work with noOpLogger, got error: %v", err)
		}

		ctx := context.Background()
		fs := NewMockFileSystem()
		err = pipeline.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected Validate to work with noOpLogger, got error: %v", err)
		}
	})

	t.Run("NoOpLogger integration with complex pipeline operations", func(t *testing.T) {
		// Test that noOpLogger handles complex logging scenarios without issues
		pipeline := execution.NewMemPipeline(nil)

		// Add operations to trigger logging
		op1 := NewMockPipelineOperation("op1", "create_directory", "parent")
		op2 := NewMockPipelineOperation("op2", "create_file", "parent/file.txt")

		err := pipeline.Add(op1, op2)
		if err != nil {
			t.Errorf("Expected Add to work with noOpLogger, got error: %v", err)
		}

		// Test resolution with logging
		err = pipeline.Resolve()
		if err != nil {
			t.Errorf("Expected Resolve to work with noOpLogger, got error: %v", err)
		}

		// Test prerequisite resolution with nil resolver
		resolver := NewMockPrerequisiteResolver()
		fs := NewMockFileSystem()
		err = pipeline.ResolvePrerequisites(resolver, fs)
		if err != nil {
			t.Errorf("Expected prerequisite resolution to work with noOpLogger, got error: %v", err)
		}

		// Test validation with complex operations
		ctx := context.Background()
		err = pipeline.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected complex validation to work with noOpLogger, got error: %v", err)
		}
	})

	t.Run("NoOpLogger stress test with multiple operations", func(t *testing.T) {
		// Create pipeline with nil logger
		pipeline := execution.NewMemPipeline(nil)

		// Add many operations to stress test the logging
		for i := 0; i < 10; i++ {
			op := NewMockPipelineOperation(fmt.Sprintf("op%d", i), "create_file", fmt.Sprintf("file%d.txt", i))

			err := pipeline.Add(op)
			if err != nil {
				t.Errorf("Expected Add operation %d to work with noOpLogger, got error: %v", i, err)
			}
		}

		// Test resolution with many operations
		err := pipeline.Resolve()
		if err != nil {
			t.Errorf("Expected Resolve with many operations to work with noOpLogger, got error: %v", err)
		}

		// Test validation with many operations
		ctx := context.Background()
		fs := NewMockFileSystem()
		err = pipeline.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected Validate with many operations to work with noOpLogger, got error: %v", err)
		}

		// Verify all operations are present
		ops := pipeline.Operations()
		if len(ops) != 10 {
			t.Errorf("Expected 10 operations, got %d", len(ops))
		}
	})

	t.Run("NoOpLogger error scenarios", func(t *testing.T) {
		// Test that noOpLogger handles error conditions gracefully
		pipeline := execution.NewMemPipeline(nil)

		// Test with invalid operations to trigger error logging paths
		err := pipeline.Add(nil)
		if err == nil {
			t.Error("Expected error when adding nil operation")
		}

		// Test duplicate ID error to trigger more logging
		op1 := NewMockPipelineOperation("duplicate", "create_file", "file1.txt")
		op2 := NewMockPipelineOperation("duplicate", "create_file", "file2.txt")

		err = pipeline.Add(op1)
		if err != nil {
			t.Errorf("Expected first Add to succeed, got error: %v", err)
		}

		err = pipeline.Add(op2)
		if err == nil {
			t.Error("Expected error when adding operation with duplicate ID")
		}

		// Test validation with operations
		op3 := NewMockPipelineOperation("op3", "create_file", "test.txt")
		op4 := NewMockPipelineOperation("op4", "delete", "test2.txt")

		pipeline2 := execution.NewMemPipeline(nil)
		err = pipeline2.Add(op3, op4)
		if err != nil {
			t.Errorf("Expected Add to succeed, got error: %v", err)
		}

		ctx := context.Background()
		fs := NewMockFileSystem()
		err = pipeline2.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected validation to succeed, got error: %v", err)
		}
	})
}

// Helper function to find index of string in slice
func findIndex(slice []string, target string) int {
	for i, item := range slice {
		if item == target {
			return i
		}
	}
	return -1
}

// Mock implementations for pipeline testing

type MockPipelineOperation struct {
	id            core.OperationID
	opType        string
	path          string
	prerequisites []core.Prerequisite
	validateError error
}

func NewMockPipelineOperation(id, opType, path string) *MockPipelineOperation {
	return &MockPipelineOperation{
		id:     core.OperationID(id),
		opType: opType,
		path:   path,
	}
}

func (m *MockPipelineOperation) ID() core.OperationID { return m.id }
func (m *MockPipelineOperation) Describe() core.OperationDesc {
	return core.OperationDesc{
		Type: m.opType,
		Path: m.path,
	}
}
func (m *MockPipelineOperation) Prerequisites() []core.Prerequisite { return m.prerequisites }
func (m *MockPipelineOperation) Dependencies() []core.OperationID { return []core.OperationID{} }
func (m *MockPipelineOperation) Conflicts() []core.OperationID { return []core.OperationID{} }
// AddDependency is no longer used as dependencies have been removed
func (m *MockPipelineOperation) AddDependency(depID core.OperationID) {
	// No-op: dependency tracking has been removed
}

func (m *MockPipelineOperation) Execute(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return nil
}

func (m *MockPipelineOperation) Validate(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return m.validateError
}

func (m *MockPipelineOperation) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	return nil, nil, nil
}

func (m *MockPipelineOperation) Rollback(ctx context.Context, fsys interface{}) error {
	return nil
}

func (m *MockPipelineOperation) GetItem() interface{}                               { return nil }
func (m *MockPipelineOperation) SetDescriptionDetail(key string, value interface{}) { /* no-op */ }

func (m *MockPipelineOperation) SetPrerequisites(prereqs []core.Prerequisite) {
	m.prerequisites = prereqs
}
func (m *MockPipelineOperation) SetValidateError(err error) { m.validateError = err }

type MockPrerequisite struct {
	prereqType    string
	path          string
	validateError error
}

func NewMockPrerequisite(prereqType, path string, validateError error) *MockPrerequisite {
	return &MockPrerequisite{
		prereqType:    prereqType,
		path:          path,
		validateError: validateError,
	}
}

func (m *MockPrerequisite) Type() string                    { return m.prereqType }
func (m *MockPrerequisite) Path() string                    { return m.path }
func (m *MockPrerequisite) Validate(fsys interface{}) error { return m.validateError }

// Reuse mock types from executor_test.go to avoid duplicates

// MockFileSystem is already defined in executor_test.go
