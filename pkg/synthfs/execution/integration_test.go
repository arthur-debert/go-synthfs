package execution

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestPrerequisiteResolutionIntegration(t *testing.T) {
	// Create a mock filesystem
	fs := &mockFileSystem{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
	}

	// Create logger
	logger := &mockLogger{}

	// Create factory and pipeline
	factory := operations.NewFactory()
	pipeline := NewMemPipeline(logger)

	// Create a file operation that requires parent directories
	op, err := factory.CreateOperation(
		core.OperationID("test-create-file"),
		"create_file",
		"parent/child/file.txt",
	)
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}

	// Add to pipeline
	if err := pipeline.Add(op); err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	// Create prerequisite resolver
	resolver := NewPrerequisiteResolver(factory, logger)

	// Resolve prerequisites
	if err := pipeline.ResolvePrerequisites(resolver, fs); err != nil {
		t.Fatalf("Failed to resolve prerequisites: %v", err)
	}

	// Check that parent directory operations were created
	ops := pipeline.Operations()
	if len(ops) < 2 {
		t.Errorf("Expected at least 2 operations (parent dir + file), got %d", len(ops))
	}

	// Verify that parent directory operations exist
	foundParentOp := false
	for _, opInterface := range ops {
		if opInterface == nil {
			continue
		}
		
		// Use interface assertion to check operation details
		if describer, ok := opInterface.(interface{ Describe() core.OperationDesc }); ok {
			desc := describer.Describe()
			if desc.Type == "create_directory" && (desc.Path == "parent" || desc.Path == "parent/child") {
				foundParentOp = true
				break
			}
		}
	}

	if !foundParentOp {
		t.Error("Expected to find parent directory operation")
	}

	t.Logf("Successfully resolved prerequisites and created %d operations", len(ops))
}

// mockFileSystem implements a simple in-memory filesystem for testing
type mockFileSystem struct {
	files map[string][]byte
	dirs  map[string]bool
}

func (fs *mockFileSystem) Stat(path string) (interface{}, error) {
	if _, exists := fs.files[path]; exists {
		return &mockFileInfo{name: path, isDir: false}, nil
	}
	if _, exists := fs.dirs[path]; exists {
		return &mockFileInfo{name: path, isDir: true}, nil
	}
	return nil, &mockPathError{op: "stat", path: path}
}

func (fs *mockFileSystem) WriteFile(name string, data []byte, perm interface{}) error {
	fs.files[name] = data
	return nil
}

func (fs *mockFileSystem) MkdirAll(path string, perm interface{}) error {
	fs.dirs[path] = true
	return nil
}

// mockFileInfo implements os.FileInfo
type mockFileInfo struct {
	name  string
	isDir bool
}

func (fi *mockFileInfo) Name() string     { return fi.name }
func (fi *mockFileInfo) Size() int64      { return 0 }
func (fi *mockFileInfo) Mode() interface{} { return 0644 }
func (fi *mockFileInfo) ModTime() interface{} { return nil }
func (fi *mockFileInfo) IsDir() bool      { return fi.isDir }
func (fi *mockFileInfo) Sys() interface{} { return nil }

// mockPathError implements error for path operations
type mockPathError struct {
	op   string
	path string
}

func (e *mockPathError) Error() string {
	return e.op + " " + e.path + ": no such file or directory"
}

// mockLogger implements core.Logger for testing
type mockLogger struct{}

func (l *mockLogger) Info() core.LogEvent  { return &mockLogEvent{} }
func (l *mockLogger) Debug() core.LogEvent { return &mockLogEvent{} }
func (l *mockLogger) Warn() core.LogEvent  { return &mockLogEvent{} }
func (l *mockLogger) Error() core.LogEvent { return &mockLogEvent{} }
func (l *mockLogger) Trace() core.LogEvent { return &mockLogEvent{} }

type mockLogEvent struct{}

func (e *mockLogEvent) Str(key, val string) core.LogEvent             { return e }
func (e *mockLogEvent) Int(key string, val int) core.LogEvent         { return e }
func (e *mockLogEvent) Bool(key string, val bool) core.LogEvent       { return e }
func (e *mockLogEvent) Dur(key string, val interface{}) core.LogEvent { return e }
func (e *mockLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *mockLogEvent) Err(err error) core.LogEvent                   { return e }
func (e *mockLogEvent) Float64(key string, val float64) core.LogEvent { return e }
func (e *mockLogEvent) Msg(msg string)                                {}