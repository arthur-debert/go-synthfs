package batch

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestMigrationPath(t *testing.T) {
	// Create a mock filesystem and registry
	mockFS := testutil.NewOperationsMockFS()
	
	// Create a simple registry for testing
	registry := &mockRegistry{}

	t.Run("Default uses old batch implementation", func(t *testing.T) {
		// Default options should use the old batch implementation
		defaultOpts := DefaultBatchOptions()
		if defaultOpts.UseSimpleBatch {
			t.Error("Default options should not use SimpleBatch for backward compatibility")
		}

		batch := NewBatchWithOptions(mockFS, registry, defaultOpts)
		
		// Verify it's the old implementation by checking type
		if _, ok := batch.(*SimpleBatchImpl); ok {
			t.Error("Default options should create BatchImpl, not SimpleBatchImpl")
		}
	})

	t.Run("UseSimpleBatch option enables new implementation", func(t *testing.T) {
		// Options with UseSimpleBatch enabled should use SimpleBatch
		opts := BatchOptions{UseSimpleBatch: true}
		batch := NewBatchWithOptions(mockFS, registry, opts)
		
		// Verify it's the new implementation
		if _, ok := batch.(*SimpleBatchImpl); !ok {
			t.Error("UseSimpleBatch option should create SimpleBatchImpl")
		}
	})

	t.Run("Both implementations support WithOptions", func(t *testing.T) {
		// Test old implementation
		oldBatch := NewBatch(mockFS, registry)
		oldBatch = oldBatch.WithOptions(BatchOptions{UseSimpleBatch: false})
		if oldBatch == nil {
			t.Error("Old batch WithOptions should not return nil")
		}

		// Test new implementation
		newBatch := NewSimpleBatch(mockFS, registry)
		newBatch = newBatch.WithOptions(BatchOptions{UseSimpleBatch: true})
		if newBatch == nil {
			t.Error("New batch WithOptions should not return nil")
		}
	})

	t.Run("Both implementations can create operations", func(t *testing.T) {
		ctx := context.Background()

		// Test old implementation
		oldBatch := NewBatch(mockFS, registry).WithContext(ctx)
		_, err := oldBatch.CreateFile("test.txt", []byte("content"))
		if err != nil {
			t.Errorf("Old batch CreateFile failed: %v", err)
		}

		// Test new implementation
		newBatch := NewSimpleBatch(mockFS, registry).WithContext(ctx)
		_, err = newBatch.CreateFile("test2.txt", []byte("content"))
		if err != nil {
			t.Errorf("New batch CreateFile failed: %v", err)
		}
	})
}

// mockRegistry is a simple registry for testing
type mockRegistry struct{}

func (r *mockRegistry) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
	return &mockOperation{
		id:     id,
		opType: opType,
		path:   path,
	}, nil
}

func (r *mockRegistry) SetItemForOperation(op interface{}, item interface{}) error {
	if mockOp, ok := op.(*mockOperation); ok {
		mockOp.item = item
	}
	return nil
}

// mockOperation is a simple operation for testing
type mockOperation struct {
	id     core.OperationID
	opType string
	path   string
	item   interface{}
	details map[string]interface{}
}

func (m *mockOperation) ID() core.OperationID {
	return m.id
}

func (m *mockOperation) Describe() core.OperationDesc {
	return core.OperationDesc{
		Type:    m.opType,
		Path:    m.path,
		Details: m.details,
	}
}

func (m *mockOperation) SetDescriptionDetail(key string, value interface{}) {
	if m.details == nil {
		m.details = make(map[string]interface{})
	}
	m.details[key] = value
}

func (m *mockOperation) Validate(ctx context.Context, fsys interface{}) error {
	return nil
}