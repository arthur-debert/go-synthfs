package synthfs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/internal/testutil"
)

// Helper mock operation for queue tests
type mockOpForQueue struct {
	id           synthfs.OperationID
	dependencies []synthfs.OperationID
	conflicts    []synthfs.OperationID
	validateErr  error
}

func newMockOpForQueue(id string) *mockOpForQueue {
	return &mockOpForQueue{
		id:           synthfs.OperationID(id),
		dependencies: []synthfs.OperationID{},
		conflicts:    []synthfs.OperationID{},
	}
}

func (m *mockOpForQueue) ID() synthfs.OperationID                           { return m.id }
func (m *mockOpForQueue) Execute(ctx context.Context, fsys synthfs.FileSystem) error { return nil }
func (m *mockOpForQueue) Validate(ctx context.Context, fsys synthfs.FileSystem) error { return m.validateErr }
func (m *mockOpForQueue) Dependencies() []synthfs.OperationID               { return m.dependencies }
func (m *mockOpForQueue) Conflicts() []synthfs.OperationID                  { return m.conflicts }
func (m *mockOpForQueue) Rollback(ctx context.Context, fsys synthfs.FileSystem) error { return nil }
func (m *mockOpForQueue) Describe() synthfs.OperationDesc {
	return synthfs.OperationDesc{Type: "mock", Path: string(m.id)}
}

func (m *mockOpForQueue) withDependency(dep string) *mockOpForQueue {
	m.dependencies = append(m.dependencies, synthfs.OperationID(dep))
	return m
}

func (m *mockOpForQueue) withConflict(conflict string) *mockOpForQueue {
	m.conflicts = append(m.conflicts, synthfs.OperationID(conflict))
	return m
}

func (m *mockOpForQueue) withValidateError(err error) *mockOpForQueue {
	m.validateErr = err
	return m
}

func TestMemQueue_Add_Operations(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	op1 := newMockOpForQueue("op1")
	op2 := newMockOpForQueue("op2")
	
	err := queue.Add(op1, op2)
	if err != nil {
		t.Errorf("Add() returned error: %v", err)
	}
	
	ops := queue.Operations()
	if len(ops) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(ops))
	}
}

func TestMemQueue_Add_DuplicateID(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	op1 := newMockOpForQueue("duplicate")
	op2 := newMockOpForQueue("duplicate")
	
	err := queue.Add(op1)
	if err != nil {
		t.Errorf("First Add() returned error: %v", err)
	}
	
	err = queue.Add(op2)
	if err == nil {
		t.Error("Expected error for duplicate ID, got nil")
	}
}

func TestMemQueue_Add_NilOperation(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	err := queue.Add(nil)
	if err == nil {
		t.Error("Expected error for nil operation, got nil")
	}
}

func TestMemQueue_Operations_ReturnsCopy(t *testing.T) {
	queue := synthfs.NewMemQueue()
	op := newMockOpForQueue("test")
	
	queue.Add(op)
	ops1 := queue.Operations()
	ops2 := queue.Operations()
	
	// Modify the slice - should not affect the queue
	ops1[0] = nil
	
	// Second call should still return valid operations
	if len(ops2) != 1 || ops2[0] == nil {
		t.Error("Operations() does not return a proper copy")
	}
}

func TestMemQueue_Resolve_SimpleDependency(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	opA := newMockOpForQueue("A")
	opB := newMockOpForQueue("B").withDependency("A")
	opC := newMockOpForQueue("C").withDependency("B")
	
	// Add in wrong order intentionally
	queue.Add(opC, opA, opB)
	
	err := queue.Resolve()
	if err != nil {
		t.Errorf("Resolve() returned error: %v", err)
	}
	
	ops := queue.Operations()
	if len(ops) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(ops))
	}
	
	// Check correct order: A -> B -> C
	expectedOrder := []synthfs.OperationID{"A", "B", "C"}
	for i, op := range ops {
		if op.ID() != expectedOrder[i] {
			t.Errorf("Operation %d: expected %s, got %s", i, expectedOrder[i], op.ID())
		}
	}
}

func TestMemQueue_Resolve_CircularDependency(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	opA := newMockOpForQueue("A").withDependency("B")
	opB := newMockOpForQueue("B").withDependency("A")
	
	queue.Add(opA, opB)
	
	err := queue.Resolve()
	if err == nil {
		t.Error("Expected error for circular dependency, got nil")
	}
}

func TestMemQueue_Resolve_MissingDependency(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	opA := newMockOpForQueue("A").withDependency("nonexistent")
	
	queue.Add(opA)
	
	err := queue.Resolve()
	if err == nil {
		t.Error("Expected error for missing dependency, got nil")
	}
	
	var depErr *synthfs.DependencyError
	if !errors.As(err, &depErr) {
		t.Errorf("Expected DependencyError, got %T", err)
	}
}

func TestMemQueue_Resolve_IndependentOperations(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	opA := newMockOpForQueue("A")
	opB := newMockOpForQueue("B")
	opC := newMockOpForQueue("C")
	
	queue.Add(opA, opB, opC)
	
	err := queue.Resolve()
	if err != nil {
		t.Errorf("Resolve() returned error: %v", err)
	}
	
	ops := queue.Operations()
	if len(ops) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(ops))
	}
	
	// Operations should maintain their original order when no dependencies
	expectedIDs := map[synthfs.OperationID]bool{"A": true, "B": true, "C": true}
	for _, op := range ops {
		if !expectedIDs[op.ID()] {
			t.Errorf("Unexpected operation ID: %s", op.ID())
		}
	}
}

func TestMemQueue_Validate_Success(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	queue := synthfs.NewMemQueue()
	
	op1 := newMockOpForQueue("op1")
	op2 := newMockOpForQueue("op2")
	
	queue.Add(op1, op2)
	
	err := queue.Validate(ctx, mfs)
	if err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}
}

func TestMemQueue_Validate_OperationValidationError(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	queue := synthfs.NewMemQueue()
	
	validationErr := errors.New("validation failed")
	op1 := newMockOpForQueue("op1")
	op2 := newMockOpForQueue("op2").withValidateError(validationErr)
	
	queue.Add(op1, op2)
	
	err := queue.Validate(ctx, mfs)
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
	
	var valErr *synthfs.ValidationError
	if !errors.As(err, &valErr) {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

func TestMemQueue_Validate_ConflictError(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	queue := synthfs.NewMemQueue()
	
	op1 := newMockOpForQueue("op1")
	op2 := newMockOpForQueue("op2").withConflict("op1")
	
	queue.Add(op1, op2)
	
	err := queue.Validate(ctx, mfs)
	if err == nil {
		t.Error("Expected conflict error, got nil")
	}
	
	var conflictErr *synthfs.ConflictError
	if !errors.As(err, &conflictErr) {
		t.Errorf("Expected ConflictError, got %T", err)
	}
}

func TestMemQueue_ComplexDependencyChain(t *testing.T) {
	queue := synthfs.NewMemQueue()
	
	// Create a complex dependency chain:
	// D -> B -> A
	// E -> C -> A  
	// F -> D, E
	opA := newMockOpForQueue("A")
	opB := newMockOpForQueue("B").withDependency("A")
	opC := newMockOpForQueue("C").withDependency("A")
	opD := newMockOpForQueue("D").withDependency("B")
	opE := newMockOpForQueue("E").withDependency("C")
	opF := newMockOpForQueue("F").withDependency("D").withDependency("E")
	
	// Add in random order
	queue.Add(opF, opC, opA, opE, opB, opD)
	
	err := queue.Resolve()
	if err != nil {
		t.Errorf("Resolve() returned error: %v", err)
	}
	
	ops := queue.Operations()
	if len(ops) != 6 {
		t.Fatalf("Expected 6 operations, got %d", len(ops))
	}
	
	// Build index of operation positions
	positions := make(map[synthfs.OperationID]int)
	for i, op := range ops {
		positions[op.ID()] = i
	}
	
	// Verify dependency constraints
	dependencies := map[synthfs.OperationID][]synthfs.OperationID{
		"B": {"A"},
		"C": {"A"},
		"D": {"B"},
		"E": {"C"}, 
		"F": {"D", "E"},
	}
	
	for op, deps := range dependencies {
		opPos := positions[op]
		for _, dep := range deps {
			depPos := positions[dep]
			if depPos >= opPos {
				t.Errorf("Dependency violation: %s (pos %d) should come before %s (pos %d)", 
					dep, depPos, op, opPos)
			}
		}
	}
}
