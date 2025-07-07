package batch

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

// MockOperationInterface for testing
type MockOperationInterface struct {
	id           core.OperationID
	desc         core.OperationDesc
	dependencies []core.OperationID
	conflicts    []core.OperationID
	item         interface{}
	details      map[string]interface{}
	srcPath      string
	dstPath      string
}

func (m *MockOperationInterface) ID() core.OperationID {
	return m.id
}

func (m *MockOperationInterface) Describe() core.OperationDesc {
	return m.desc
}

func (m *MockOperationInterface) Dependencies() []core.OperationID {
	return m.dependencies
}

func (m *MockOperationInterface) Conflicts() []core.OperationID {
	return m.conflicts
}

func (m *MockOperationInterface) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return nil
}

func (m *MockOperationInterface) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return nil
}

func (m *MockOperationInterface) Validate(ctx context.Context, fsys FilesystemInterface) error {
	return nil
}

func (m *MockOperationInterface) SetDescriptionDetail(key string, value interface{}) {
	if m.details == nil {
		m.details = make(map[string]interface{})
	}
	m.details[key] = value
}

func (m *MockOperationInterface) AddDependency(depID core.OperationID) {
	m.dependencies = append(m.dependencies, depID)
}

func (m *MockOperationInterface) SetPaths(src, dst string) {
	m.srcPath = src
	m.dstPath = dst
}

func (m *MockOperationInterface) GetItem() interface{} {
	return m.item
}

// MockFilesystemInterface for testing
type MockFilesystemInterface struct {
	files map[string]interface{}
}

func NewMockFilesystemInterface() *MockFilesystemInterface {
	return &MockFilesystemInterface{
		files: make(map[string]interface{}),
	}
}

func (m *MockFilesystemInterface) Stat(name string) (interface{}, error) {
	if _, exists := m.files[name]; exists {
		return &mockFileInfo{name: name}, nil
	}
	return nil, fs.ErrNotExist
}

func (m *MockFilesystemInterface) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.files[name] = data
	return nil
}

func (m *MockFilesystemInterface) MkdirAll(path string, perm fs.FileMode) error {
	m.files[path] = "directory"
	return nil
}

func (m *MockFilesystemInterface) Remove(name string) error {
	delete(m.files, name)
	return nil
}

func (m *MockFilesystemInterface) RemoveAll(name string) error {
	delete(m.files, name)
	return nil
}

func (m *MockFilesystemInterface) Rename(oldpath, newpath string) error {
	if data, exists := m.files[oldpath]; exists {
		m.files[newpath] = data
		delete(m.files, oldpath)
	}
	return nil
}

func (m *MockFilesystemInterface) Symlink(oldname, newname string) error {
	m.files[newname] = "symlink:" + oldname
	return nil
}

type mockFileInfo struct {
	name string
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// MockPathStateTrackerInterface for testing
type MockPathStateTrackerInterface struct {
	deletedPaths map[string]bool
}

func NewMockPathStateTrackerInterface() *MockPathStateTrackerInterface {
	return &MockPathStateTrackerInterface{
		deletedPaths: make(map[string]bool),
	}
}

func (m *MockPathStateTrackerInterface) UpdateState(op OperationInterface) error {
	if op.Describe().Type == "delete" {
		m.deletedPaths[op.Describe().Path] = true
	}
	return nil
}

func (m *MockPathStateTrackerInterface) IsDeleted(path string) bool {
	return m.deletedPaths[path]
}

// MockOperationFactory for testing
type MockOperationFactory struct{}

func (m *MockOperationFactory) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
	return &MockOperationInterface{
		id: id,
		desc: core.OperationDesc{
			Type: opType,
			Path: path,
		},
		dependencies: []core.OperationID{},
		conflicts:    []core.OperationID{},
		details:      make(map[string]interface{}),
	}, nil
}

func (m *MockOperationFactory) SetItemForOperation(op interface{}, item interface{}) error {
	if mockOp, ok := op.(*MockOperationInterface); ok {
		mockOp.item = item
	}
	return nil
}

// MockLogger for testing
type MockLogger struct{}

func (m *MockLogger) Info() core.LogEvent  { return &MockLogEvent{} }
func (m *MockLogger) Debug() core.LogEvent { return &MockLogEvent{} }
func (m *MockLogger) Warn() core.LogEvent  { return &MockLogEvent{} }
func (m *MockLogger) Error() core.LogEvent { return &MockLogEvent{} }
func (m *MockLogger) Trace() core.LogEvent { return &MockLogEvent{} }

type MockLogEvent struct{}

func (e *MockLogEvent) Str(key, val string) core.LogEvent                   { return e }
func (e *MockLogEvent) Int(key string, val int) core.LogEvent               { return e }
func (e *MockLogEvent) Err(err error) core.LogEvent                         { return e }
func (e *MockLogEvent) Float64(key string, val float64) core.LogEvent       { return e }
func (e *MockLogEvent) Bool(key string, val bool) core.LogEvent             { return e }
func (e *MockLogEvent) Dur(key string, val interface{}) core.LogEvent       { return e }
func (e *MockLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *MockLogEvent) Msg(msg string)                                      {}

func TestNewBatch(t *testing.T) {
	fs := NewMockFilesystemInterface()
	registry := &MockOperationFactory{}
	logger := &MockLogger{}
	pathTracker := NewMockPathStateTrackerInterface()

	batch := NewBatch(fs, registry, logger, pathTracker)

	if batch == nil {
		t.Fatal("NewBatch returned nil")
	}

	if len(batch.Operations()) != 0 {
		t.Errorf("Expected empty batch, got %d operations", len(batch.Operations()))
	}
}

func TestBatchCreateDir(t *testing.T) {
	fs := NewMockFilesystemInterface()
	registry := &MockOperationFactory{}
	logger := &MockLogger{}
	pathTracker := NewMockPathStateTrackerInterface()

	batch := NewBatch(fs, registry, logger, pathTracker)

	op, err := batch.CreateDir("testdir")
	if err != nil {
		t.Fatalf("Failed to create directory operation: %v", err)
	}

	if op == nil {
		t.Fatal("CreateDir returned nil operation")
	}

	if op.Describe().Type != "create_directory" {
		t.Errorf("Expected operation type 'create_directory', got '%s'", op.Describe().Type)
	}

	if op.Describe().Path != "testdir" {
		t.Errorf("Expected path 'testdir', got '%s'", op.Describe().Path)
	}

	// Check that operation was added to batch
	ops := batch.Operations()
	if len(ops) != 1 {
		t.Errorf("Expected 1 operation in batch, got %d", len(ops))
	}
}

func TestBatchCreateFile(t *testing.T) {
	fs := NewMockFilesystemInterface()
	registry := &MockOperationFactory{}
	logger := &MockLogger{}
	pathTracker := NewMockPathStateTrackerInterface()

	batch := NewBatch(fs, registry, logger, pathTracker)

	content := []byte("test content")
	op, err := batch.CreateFile("testfile.txt", content)
	if err != nil {
		t.Fatalf("Failed to create file operation: %v", err)
	}

	if op == nil {
		t.Fatal("CreateFile returned nil operation")
	}

	if op.Describe().Type != "create_file" {
		t.Errorf("Expected operation type 'create_file', got '%s'", op.Describe().Type)
	}

	if op.Describe().Path != "testfile.txt" {
		t.Errorf("Expected path 'testfile.txt', got '%s'", op.Describe().Path)
	}

	// Check that the item was set correctly
	item := op.GetItem()
	if fileItem, ok := item.(*targets.FileItem); ok {
		if string(fileItem.Content()) != string(content) {
			t.Errorf("Expected content '%s', got '%s'", string(content), string(fileItem.Content()))
		}
	} else {
		t.Errorf("Expected FileItem, got %T", item)
	}
}

func TestBatchCopy(t *testing.T) {
	fs := NewMockFilesystemInterface()
	registry := &MockOperationFactory{}
	logger := &MockLogger{}
	pathTracker := NewMockPathStateTrackerInterface()

	batch := NewBatch(fs, registry, logger, pathTracker)

	op, err := batch.Copy("source.txt", "dest.txt")
	if err != nil {
		t.Fatalf("Failed to create copy operation: %v", err)
	}

	if op.Describe().Type != "copy" {
		t.Errorf("Expected operation type 'copy', got '%s'", op.Describe().Type)
	}

	if op.Describe().Path != "source.txt" {
		t.Errorf("Expected path 'source.txt', got '%s'", op.Describe().Path)
	}

	// Check that paths were set
	mockOp := op.(*MockOperationInterface)
	if mockOp.srcPath != "source.txt" {
		t.Errorf("Expected source path 'source.txt', got '%s'", mockOp.srcPath)
	}
	if mockOp.dstPath != "dest.txt" {
		t.Errorf("Expected destination path 'dest.txt', got '%s'", mockOp.dstPath)
	}
}

func TestBatchWithMethods(t *testing.T) {
	fs := NewMockFilesystemInterface()
	registry := &MockOperationFactory{}
	logger := &MockLogger{}
	pathTracker := NewMockPathStateTrackerInterface()

	batch := NewBatch(fs, registry, logger, pathTracker)

	// Test WithContext
	ctx := context.Background()
	batch = batch.WithContext(ctx)
	if batch.ctx != ctx {
		t.Error("WithContext did not set context correctly")
	}

	// Test WithRegistry
	newRegistry := &MockOperationFactory{}
	batch = batch.WithRegistry(newRegistry)
	if batch.registry != newRegistry {
		t.Error("WithRegistry did not set registry correctly")
	}

	// Test WithFileSystem
	newFS := NewMockFilesystemInterface()
	batch = batch.WithFileSystem(newFS)
	if batch.fs != newFS {
		t.Error("WithFileSystem did not set filesystem correctly")
	}
}