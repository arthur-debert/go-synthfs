package execution

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// MockOperationFactory for testing
type MockOperationFactory struct {
	createdOps map[string]interface{}
}

func NewMockOperationFactory() *MockOperationFactory {
	return &MockOperationFactory{
		createdOps: make(map[string]interface{}),
	}
}

func (f *MockOperationFactory) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
	op := &MockOperation{
		id:   id,
		path: path,
		opType: opType,
	}
	f.createdOps[string(id)] = op
	return op, nil
}

func (f *MockOperationFactory) SetItemForOperation(op interface{}, item interface{}) error {
	if mockOp, ok := op.(*MockOperation); ok {
		mockOp.item = item
	}
	return nil
}

// MockOperation for testing
type MockOperation struct {
	id     core.OperationID
	path   string
	opType string
	item   interface{}
}

func (op *MockOperation) ID() core.OperationID {
	return op.id
}

func (op *MockOperation) Describe() core.OperationDesc {
	return core.OperationDesc{
		Type: op.opType,
		Path: op.path,
	}
}

func (op *MockOperation) GetItem() interface{} {
	return op.item
}

func TestDefaultPrerequisiteResolver(t *testing.T) {
	factory := NewMockOperationFactory()
	resolver := NewDefaultPrerequisiteResolver(factory)
	
	t.Run("CanResolve parent_dir", func(t *testing.T) {
		prereq := core.NewParentDirPrerequisite("dir/file.txt")
		
		if !resolver.CanResolve(prereq) {
			t.Error("Expected resolver to be able to resolve parent_dir prerequisite")
		}
	})
	
	t.Run("Cannot resolve no_conflict", func(t *testing.T) {
		prereq := core.NewNoConflictPrerequisite("file.txt")
		
		if resolver.CanResolve(prereq) {
			t.Error("Expected resolver to NOT be able to resolve no_conflict prerequisite")
		}
	})
	
	t.Run("Cannot resolve source_exists", func(t *testing.T) {
		prereq := core.NewSourceExistsPrerequisite("file.txt")
		
		if resolver.CanResolve(prereq) {
			t.Error("Expected resolver to NOT be able to resolve source_exists prerequisite")
		}
	})
	
	t.Run("Resolve parent_dir creates directory operation", func(t *testing.T) {
		prereq := core.NewParentDirPrerequisite("parent/child/file.txt")
		
		ops, err := resolver.Resolve(prereq)
		if err != nil {
			t.Fatalf("Failed to resolve parent_dir prerequisite: %v", err)
		}
		
		if len(ops) != 1 {
			t.Fatalf("Expected 1 operation, got %d", len(ops))
		}
		
		// Check the created operation
		if mockOp, ok := ops[0].(*MockOperation); ok {
			if mockOp.opType != "create_directory" {
				t.Errorf("Expected create_directory operation, got %s", mockOp.opType)
			}
			
			if mockOp.path != "parent/child" {
				t.Errorf("Expected path 'parent/child', got '%s'", mockOp.path)
			}
			
			// Check that item was set
			if mockOp.item == nil {
				t.Error("Expected item to be set on operation")
			} else if dirItem, ok := mockOp.item.(*DirectoryItem); ok {
				if dirItem.Path() != "parent/child" {
					t.Errorf("Expected directory item path 'parent/child', got '%s'", dirItem.Path())
				}
				
				if dirItem.Type() != "directory" {
					t.Errorf("Expected directory item type 'directory', got '%s'", dirItem.Type())
				}
				
				if !dirItem.IsDir() {
					t.Error("Expected directory item IsDir() to return true")
				}
			} else {
				t.Errorf("Expected DirectoryItem, got %T", mockOp.item)
			}
		} else {
			t.Errorf("Expected MockOperation, got %T", ops[0])
		}
	})
	
	t.Run("Resolve root directory returns no operations", func(t *testing.T) {
		prereq := core.NewParentDirPrerequisite("/file.txt")
		
		ops, err := resolver.Resolve(prereq)
		if err != nil {
			t.Fatalf("Failed to resolve root directory prerequisite: %v", err)
		}
		
		if len(ops) != 0 {
			t.Errorf("Expected 0 operations for root directory, got %d", len(ops))
		}
	})
	
	t.Run("Resolve current directory returns no operations", func(t *testing.T) {
		prereq := core.NewParentDirPrerequisite("file.txt")
		
		ops, err := resolver.Resolve(prereq)
		if err != nil {
			t.Fatalf("Failed to resolve current directory prerequisite: %v", err)
		}
		
		if len(ops) != 0 {
			t.Errorf("Expected 0 operations for current directory, got %d", len(ops))
		}
	})
	
	t.Run("Resolve unsupported prerequisite returns error", func(t *testing.T) {
		prereq := core.NewNoConflictPrerequisite("file.txt")
		
		_, err := resolver.Resolve(prereq)
		if err == nil {
			t.Error("Expected error when resolving unsupported prerequisite")
		}
	})
}

func TestDirectoryItem(t *testing.T) {
	item := &DirectoryItem{
		path: "test/dir",
		mode: 0755,
	}
	
	t.Run("DirectoryItem methods", func(t *testing.T) {
		if item.Path() != "test/dir" {
			t.Errorf("Expected path 'test/dir', got '%s'", item.Path())
		}
		
		if item.Type() != "directory" {
			t.Errorf("Expected type 'directory', got '%s'", item.Type())
		}
		
		if !item.IsDir() {
			t.Error("Expected IsDir() to return true")
		}
		
		if item.Mode() != 0755 {
			t.Errorf("Expected mode 0755, got %o", item.Mode())
		}
	})
}

func TestGeneratePathID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"path/with/slashes", "path_with_slashes"},
		{"path\\with\\backslashes", "path_with_backslashes"},
		{"path:with:colons", "path_with_colons"},
		{"path with spaces", "path_with_spaces"},
		{"complex/path\\with:multiple characters", "complex_path_with_multiple_characters"},
	}
	
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := generatePathID(test.input)
			if result != test.expected {
				t.Errorf("Expected '%s', got '%s'", test.expected, result)
			}
		})
	}
}

func TestNewPrerequisiteResolver(t *testing.T) {
	factory := NewMockOperationFactory()
	logger := &MockLogger{}
	
	// Test the compatibility function
	resolver := NewPrerequisiteResolver(factory, logger)
	
	if resolver == nil {
		t.Fatal("Expected resolver to be created")
	}
	
	// Test that it can resolve parent_dir prerequisites
	prereq := core.NewParentDirPrerequisite("dir/file.txt")
	if !resolver.CanResolve(prereq) {
		t.Error("Expected resolver to be able to resolve parent_dir prerequisite")
	}
}

// MockLogger for testing
type MockLogger struct{}

func (l *MockLogger) Info() core.LogEvent  { return &MockLogEvent{} }
func (l *MockLogger) Debug() core.LogEvent { return &MockLogEvent{} }
func (l *MockLogger) Warn() core.LogEvent  { return &MockLogEvent{} }
func (l *MockLogger) Error() core.LogEvent { return &MockLogEvent{} }
func (l *MockLogger) Trace() core.LogEvent { return &MockLogEvent{} }

type MockLogEvent struct{}

func (e *MockLogEvent) Str(key, val string) core.LogEvent                   { return e }
func (e *MockLogEvent) Int(key string, val int) core.LogEvent               { return e }
func (e *MockLogEvent) Bool(key string, val bool) core.LogEvent             { return e }
func (e *MockLogEvent) Dur(key string, val interface{}) core.LogEvent       { return e }
func (e *MockLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *MockLogEvent) Err(err error) core.LogEvent                         { return e }
func (e *MockLogEvent) Float64(key string, val float64) core.LogEvent       { return e }
func (e *MockLogEvent) Msg(msg string)                                      {}