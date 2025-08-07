package testutil

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// TestFileSystem is now a type alias for the filesystem package version
type TestFileSystem = filesystem.TestFileSystem

// NewTestFileSystem creates a new test filesystem based on fstest.MapFS
func NewTestFileSystem() *TestFileSystem {
	return filesystem.NewTestFileSystem()
}

// NewTestFileSystemFromMap creates a test filesystem from an existing map
func NewTestFileSystemFromMap(files map[string]*fstest.MapFile) *TestFileSystem {
	return filesystem.NewTestFileSystemFromMap(files)
}

// TestHelper is now a type alias for the filesystem package version
type TestHelper = filesystem.TestHelper

// NewTestHelper creates a new test helper with a fresh filesystem
func NewTestHelper(t *testing.T) *TestHelper {
	return filesystem.NewTestHelper(t)
}

// NewTestHelperWithFiles creates a test helper with predefined files
func NewTestHelperWithFiles(t *testing.T, files map[string]*fstest.MapFile) *TestHelper {
	return filesystem.NewTestHelperWithFiles(t, files)
}

// RunOperationTest runs a test for a single operation
func RunOperationTest(t *testing.T, name string, test func(t *testing.T, fs synthfs.FileSystem, ctx context.Context)) {
	t.Run(name, func(t *testing.T) {
		fs := NewTestFileSystem()
		ctx := context.Background()
		test(t, fs, ctx)
	})
}

// ValidateOperation validates an operation and returns any error
func ValidateOperation(t *testing.T, op synthfs.Operation, fs synthfs.FileSystem) error {
	ctx := context.Background()
	return op.Validate(ctx, nil, fs)
}

// ExecuteOperation executes an operation and returns any error
func ExecuteOperation(t *testing.T, op synthfs.Operation, fs synthfs.FileSystem) error {
	ctx := context.Background()
	return op.Execute(ctx, nil, fs)
}

// RunPipelineTest runs a test for a pipeline of operations
func RunPipelineTest(t *testing.T, name string, test func(t *testing.T, p synthfs.Pipeline, fs synthfs.FileSystem, ctx context.Context)) {
	t.Run(name, func(t *testing.T) {
		fs := NewTestFileSystem()
		ctx := context.Background()
		p := synthfs.NewMemPipeline()
		test(t, p, fs, ctx)
	})
}

// AssertOperation is a helper to assert that an operation succeeds
func AssertOperation(t *testing.T, op synthfs.Operation, fs synthfs.FileSystem, msg string) {
	ctx := context.Background()
	if err := op.Validate(ctx, nil, fs); err != nil {
		t.Errorf("%s: validation failed: %v", msg, err)
		return
	}
	if err := op.Execute(ctx, nil, fs); err != nil {
		t.Errorf("%s: execution failed: %v", msg, err)
	}
}

// AssertOperationFails is a helper to assert that an operation fails
func AssertOperationFails(t *testing.T, op synthfs.Operation, fs synthfs.FileSystem, stage string, msg string) {
	ctx := context.Background()
	switch stage {
	case "validate":
		if err := op.Validate(ctx, nil, fs); err == nil {
			t.Errorf("%s: expected validation to fail", msg)
		}
	case "execute":
		if err := op.Execute(ctx, nil, fs); err == nil {
			t.Errorf("%s: expected execution to fail", msg)
		}
	default:
		t.Fatalf("Invalid stage: %s", stage)
	}
}

// CreateTestFile is a helper to create a file with content
func CreateTestFile(t *testing.T, fs synthfs.FileSystem, path string, content []byte) {
	if err := fs.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", path, err)
	}
}

// CreateTestDir is a helper to create a directory
func CreateTestDir(t *testing.T, fs synthfs.FileSystem, path string) {
	if err := fs.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create test directory %s: %v", path, err)
	}
}

// FileExists checks if a file exists in the filesystem
func FileExists(t *testing.T, fs synthfs.FileSystem, path string) bool {
	_, err := fs.Stat(path)
	return err == nil
}

// AssertFileContent verifies that a file has the expected content
func AssertFileContent(t *testing.T, fs synthfs.FileSystem, path string, expected []byte) {
	file, err := fs.Open(path)
	if err != nil {
		t.Errorf("Failed to open file %s: %v", path, err)
		return
	}
	defer func() {
		_ = file.Close() // Best effort close
	}()

	info, err := file.Stat()
	if err != nil {
		t.Errorf("Failed to stat file %s: %v", path, err)
		return
	}

	actual := make([]byte, info.Size())
	n, err := file.Read(actual)
	if err != nil {
		t.Errorf("Failed to read file %s: %v", path, err)
		return
	}

	if string(actual[:n]) != string(expected) {
		t.Errorf("File %s content mismatch:\nExpected: %q\nActual: %q", path, expected, actual[:n])
	}
}

// SetupTestFiles creates multiple test files from a map
func SetupTestFiles(t *testing.T, fs synthfs.FileSystem, files map[string]string) {
	for path, content := range files {
		CreateTestFile(t, fs, path, []byte(content))
	}
}

// SetupTestDirs creates multiple test directories from a slice
func SetupTestDirs(t *testing.T, fs synthfs.FileSystem, dirs []string) {
	for _, dir := range dirs {
		CreateTestDir(t, fs, dir)
	}
}

// AssertFileNotExists verifies that a file does not exist
func AssertFileNotExists(t *testing.T, fs synthfs.FileSystem, path string) {
	if FileExists(t, fs, path) {
		t.Errorf("Expected file %s to not exist, but it does", path)
	}
}

// LogOperationDetails logs details about an operation for debugging
func LogOperationDetails(t *testing.T, op synthfs.Operation) {
	desc := op.Describe()
	t.Logf("Operation: ID=%s, Type=%s, Path=%s", op.ID(), desc.Type, desc.Path)
	if len(desc.Details) > 0 {
		t.Logf("Details: %v", desc.Details)
	}
}

// TestBatchHelper provides utilities for testing batch operations
type TestBatchHelper struct {
	t     *testing.T
	batch *synthfs.Batch
	fs    synthfs.FileSystem
}

// NewTestBatchHelper creates a new batch test helper
func NewTestBatchHelper(t *testing.T) *TestBatchHelper {
	fs := NewTestFileSystem()
	batch := synthfs.NewBatch(fs)
	return &TestBatchHelper{
		t:     t,
		batch: batch,
		fs:    fs,
	}
}

// Batch returns the test batch
func (tbh *TestBatchHelper) Batch() *synthfs.Batch {
	return tbh.batch
}

// FileSystem returns the test filesystem
func (tbh *TestBatchHelper) FileSystem() synthfs.FileSystem {
	return tbh.fs
}

// Run executes the batch and returns the result
func (tbh *TestBatchHelper) Run() (*synthfs.Result, error) {
	result, err := tbh.batch.Run()
	if err != nil {
		tbh.t.Logf("Batch run failed: %v", err)
	}
	return result, err
}

// AssertSuccess asserts that the batch runs successfully
func (tbh *TestBatchHelper) AssertSuccess() *synthfs.Result {
	result, err := tbh.Run()
	if err != nil {
		tbh.t.Fatalf("Expected batch to succeed, but got error: %v", err)
	}
	if !result.IsSuccess() {
		tbh.t.Fatalf("Expected batch to succeed, but Success=false. Error: %v", result.GetError())
	}
	return result
}

// AssertFailure asserts that the batch fails
func (tbh *TestBatchHelper) AssertFailure() *synthfs.Result {
	result, err := tbh.Run()
	if err == nil && result.IsSuccess() {
		tbh.t.Fatalf("Expected batch to fail, but it succeeded")
	}
	return result
}

// TestPipelineHelper provides utilities for testing pipelines
type TestPipelineHelper struct {
	t        *testing.T
	pipeline synthfs.Pipeline
	fs       synthfs.FileSystem
	executor *synthfs.Executor
}

// NewTestPipelineHelper creates a new pipeline test helper
func NewTestPipelineHelper(t *testing.T) *TestPipelineHelper {
	fs := NewTestFileSystem()
	pipeline := synthfs.NewMemPipeline()
	executor := synthfs.NewExecutor()
	return &TestPipelineHelper{
		t:        t,
		pipeline: pipeline,
		fs:       fs,
		executor: executor,
	}
}

// Pipeline returns the test pipeline
func (tph *TestPipelineHelper) Pipeline() synthfs.Pipeline {
	return tph.pipeline
}

// FileSystem returns the test filesystem
func (tph *TestPipelineHelper) FileSystem() synthfs.FileSystem {
	return tph.fs
}

// Execute runs the pipeline and returns the result
func (tph *TestPipelineHelper) Execute(ctx context.Context) *synthfs.Result {
	return tph.executor.Run(ctx, tph.pipeline, tph.fs)
}

// ExecuteWithOptions runs the pipeline with options and returns the result
func (tph *TestPipelineHelper) ExecuteWithOptions(ctx context.Context, opts synthfs.PipelineOptions) *synthfs.Result {
	return tph.executor.RunWithOptions(ctx, tph.pipeline, tph.fs, opts)
}

// AssertSuccess asserts that the pipeline executes successfully
func (tph *TestPipelineHelper) AssertSuccess(ctx context.Context) *synthfs.Result {
	result := tph.Execute(ctx)
	if !result.IsSuccess() {
		tph.t.Fatalf("Expected pipeline to succeed, but Success=false. Error: %v", result.GetError())
	}
	return result
}

// AssertFailure asserts that the pipeline fails
func (tph *TestPipelineHelper) AssertFailure(ctx context.Context) *synthfs.Result {
	result := tph.Execute(ctx)
	if result.IsSuccess() {
		tph.t.Fatalf("Expected pipeline to fail, but it succeeded")
	}
	return result
}

// AssertOperationCount asserts the number of operations in the pipeline
func (tph *TestPipelineHelper) AssertOperationCount(expected int) {
	actual := len(tph.pipeline.Operations())
	if actual != expected {
		tph.t.Errorf("Expected %d operations, got %d", expected, actual)
	}
}

// AddOperation adds an operation to the pipeline
func (tph *TestPipelineHelper) AddOperation(op synthfs.Operation) {
	if err := tph.pipeline.Add(op); err != nil {
		tph.t.Errorf("Failed to add operation to pipeline: %v", err)
	}
}

// LogPipelineState logs the current state of the pipeline for debugging
func (tph *TestPipelineHelper) LogPipelineState() {
	ops := tph.pipeline.Operations()
	tph.t.Logf("Pipeline contains %d operations:", len(ops))
	for i, op := range ops {
		desc := op.Describe()
		tph.t.Logf("  [%d] ID=%s, Type=%s, Path=%s", i, op.ID(), desc.Type, desc.Path)
	}
}

// CreateTestOperation creates a simple test operation
func CreateTestOperation(id, opType, path string) synthfs.Operation {
	registry := synthfs.NewOperationRegistry()
	op, err := registry.CreateOperation(synthfs.OperationID(id), opType, path)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test operation: %v", err))
	}
	return op.(synthfs.Operation)
}

// CreateTestFileOperation creates a file operation with content
func CreateTestFileOperation(id, path string, content []byte) synthfs.Operation {
	op := CreateTestOperation(id, "create_file", path)
	item := synthfs.NewFile(path).WithContent(content)
	if err := synthfs.NewOperationRegistry().SetItemForOperation(op, item); err != nil {
		panic(fmt.Sprintf("Failed to set item for operation: %v", err))
	}
	return op
}

// CreateTestDirectoryOperation creates a directory operation
func CreateTestDirectoryOperation(id, path string) synthfs.Operation {
	op := CreateTestOperation(id, "create_directory", path)
	item := synthfs.NewDirectory(path)
	if err := synthfs.NewOperationRegistry().SetItemForOperation(op, item); err != nil {
		panic(fmt.Sprintf("Failed to set item for operation: %v", err))
	}
	return op
}

// CreateTestCopyOperation creates a copy operation
func CreateTestCopyOperation(id, src, dst string) synthfs.Operation {
	op := CreateTestOperation(id, "copy", src)
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)
	return op
}

// CreateTestMoveOperation creates a move operation
func CreateTestMoveOperation(id, src, dst string) synthfs.Operation {
	op := CreateTestOperation(id, "move", src)
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)
	return op
}

// CreateTestDeleteOperation creates a delete operation
func CreateTestDeleteOperation(id, path string) synthfs.Operation {
	return CreateTestOperation(id, "delete", path)
}
