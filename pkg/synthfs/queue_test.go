package synthfs_test // Testing synthfs package, so use synthfs_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/ops" // For concrete operation types
)

// mockOperation is a simple operation for testing the queue.
type mockOperation struct {
	id           synthfs.OperationID
	dependencies []synthfs.OperationID
	description  string
}

func newMockOp(id string) *mockOperation {
	return &mockOperation{id: synthfs.OperationID(id), description: "mock op " + id}
}
func (m *mockOperation) ID() synthfs.OperationID                            { return m.id }
func (m *mockOperation) Execute(ctx context.Context, fsys synthfs.FileSystem) error { return nil }
func (m *mockOperation) Validate(ctx context.Context, fsys synthfs.FileSystem) error { return nil }
func (m *mockOperation) Dependencies() []synthfs.OperationID                  { return m.dependencies }
func (m *mockOperation) Conflicts() []synthfs.OperationID                     { return nil }
func (m *mockOperation) Rollback(ctx context.Context, fsys synthfs.FileSystem) error { return nil }
func (m *mockOperation) Describe() synthfs.OperationDesc {
	return synthfs.OperationDesc{Type: "mock", Path: m.description}
}

func TestMemQueue_Add_Operations(t *testing.T) {
	q := synthfs.NewMemQueue()

	op1 := newMockOp("op1")
	op2 := newMockOp("op2")

	err := q.Add(op1)
	if err != nil {
		t.Fatalf("Add(op1) failed: %v", err)
	}
	err = q.Add(op2)
	if err != nil {
		t.Fatalf("Add(op2) failed: %v", err)
	}

	queuedOps := q.Operations()
	if len(queuedOps) != 2 {
		t.Fatalf("Operations() count = %d, want 2", len(queuedOps))
	}
	if queuedOps[0].ID() != op1.ID() || queuedOps[1].ID() != op2.ID() {
		t.Errorf("Operations() order or content mismatch: got IDs %s, %s; want %s, %s",
			queuedOps[0].ID(), queuedOps[1].ID(), op1.ID(), op2.ID())
	}

	// Test adding multiple operations at once
	q2 := synthfs.NewMemQueue()
	op3 := newMockOp("op3")
	op4 := newMockOp("op4")
	err = q2.Add(op3, op4)
	if err != nil {
		t.Fatalf("Add(op3, op4) failed: %v", err)
	}
	queuedOps2 := q2.Operations()
	if len(queuedOps2) != 2 {
		t.Fatalf("Operations() count after multi-add = %d, want 2", len(queuedOps2))
	}
	if queuedOps2[0].ID() != op3.ID() || queuedOps2[1].ID() != op4.ID() {
		t.Errorf("Operations() order or content mismatch after multi-add")
	}
}

func TestMemQueue_Add_DuplicateID(t *testing.T) {
	q := synthfs.NewMemQueue()
	op1 := newMockOp("op1")
	op1Dup := newMockOp("op1") // Same ID

	err := q.Add(op1)
	if err != nil {
		t.Fatalf("Add(op1) failed: %v", err)
	}

	err = q.Add(op1Dup)
	if err == nil {
		t.Errorf("Add(op1Dup) expected error for duplicate ID, got nil")
	}
	expectedErr := fmt.Sprintf("operation with ID '%s' already exists in the queue", op1.ID())
	if err != nil && err.Error() != expectedErr {
		t.Errorf("Add(op1Dup) error = %q, want %q", err.Error(), expectedErr)
	}

	// Ensure only the first op1 was added
	queuedOps := q.Operations()
	if len(queuedOps) != 1 {
		t.Errorf("Operations() count = %d, want 1 after duplicate add attempt", len(queuedOps))
	}
}

func TestMemQueue_Add_NilOperation(t *testing.T) {
	q := synthfs.NewMemQueue()
	err := q.Add(nil)
	if err == nil {
		t.Errorf("Add(nil) expected error, got nil")
	}
	expectedErr := "cannot add a nil operation to the queue"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("Add(nil) error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestMemQueue_Operations_ReturnsCopy(t *testing.T) {
	q := synthfs.NewMemQueue()
	op1 := ops.NewCreateFile("file.txt", []byte("data"), 0644).WithID("op1-cf") // Using a concrete op

	q.Add(op1)

	ops1 := q.Operations()
	if len(ops1) != 1 {
		t.Fatalf("Expected 1 op, got %d", len(ops1))
	}
	// Modify the returned slice (should not affect the queue's internal slice)
	ops1 = append(ops1, ops.NewCreateDir("dir", 0755).WithID("op2-cd"))


	ops2 := q.Operations()
	if len(ops2) != 1 {
		t.Errorf("Operations() count = %d, want 1; modification of returned slice affected internal queue", len(ops2))
	}
}
