package batch

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// logInfo is a simple logger for the batch package
func logInfo(msg string, fields map[string]interface{}) {
	// For now, just use fmt for logging
	// TODO: Integrate with proper logging system
	fmt.Printf("[BATCH] %s %+v\n", msg, fields)
}


// BatchImpl represents a collection of operations that can be validated and executed as a unit.
// This implementation uses interface{} types to avoid circular dependencies.
type BatchImpl struct {
	operations []interface{}
	fs         interface{} // Filesystem interface
	ctx        context.Context
	idCounter  int
	// TODO: Add pathTracker when implementing path state tracking
	// pathTracker interface{} // Path state tracker interface
	registry core.OperationFactory
}

// NewBatch creates a new operation batch.
func NewBatch(fs interface{}, registry core.OperationFactory) Batch {
	return &BatchImpl{
		operations: []interface{}{},
		fs:         fs,
		ctx:        context.Background(),
		idCounter:  0,
		registry:   registry,
	}
}

// Operations returns all operations currently in the batch.
func (b *BatchImpl) Operations() []interface{} {
	// Return a copy to prevent external modification
	opsCopy := make([]interface{}, len(b.operations))
	copy(opsCopy, b.operations)
	return opsCopy
}

// WithFileSystem sets the filesystem for the batch operations.
func (b *BatchImpl) WithFileSystem(fs interface{}) Batch {
	b.fs = fs
	// TODO: Recreate pathTracker with new filesystem
	return b
}

// WithContext sets the context for the batch operations.
func (b *BatchImpl) WithContext(ctx context.Context) Batch {
	b.ctx = ctx
	return b
}

// WithRegistry sets a custom operation registry for the batch.
func (b *BatchImpl) WithRegistry(registry core.OperationFactory) Batch {
	b.registry = registry
	return b
}

// add adds an operation to the batch and validates it
func (b *BatchImpl) add(op interface{}) error {
	// Get validation method through interface assertion
	type validator interface {
		Validate(ctx context.Context, fsys interface{}) error
	}

	if v, ok := op.(validator); ok {
		if err := v.Validate(b.ctx, b.fs); err != nil {
			return err
		}
	}

	// TODO: Validate against projected state using pathTracker

	b.operations = append(b.operations, op)
	return nil
}

// CreateDir adds a directory creation operation to the batch.
func (b *BatchImpl) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0755) // Default directory mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation using the registry
	op, err := b.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	// Set the item for this create operation
	// Note: The actual directory item creation is handled by the main package
	// This is just setting up the operation metadata
	if err := b.setOperationDetails(op, map[string]interface{}{
		"mode": fileMode.String(),
	}); err != nil {
		return nil, err
	}

	// Add to batch (which validates)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
func (b *BatchImpl) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0644) // Default file mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation
	op, err := b.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"content_length": len(content),
		"mode":           fileMode.String(),
		"content":        content,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	return op, nil
}

// Copy adds a copy operation to the batch.
func (b *BatchImpl) Copy(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	return op, nil
}

// Move adds a move operation to the batch.
func (b *BatchImpl) Move(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	return op, nil
}

// Delete adds a delete operation to the batch.
func (b *BatchImpl) Delete(path string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete(%s): %w", path, err)
	}

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (b *BatchImpl) CreateSymlink(target, linkPath string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"target": target,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
func (b *BatchImpl) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
	// Validate inputs
	if len(sources) == 0 {
		return nil, fmt.Errorf("validation failed for CreateArchive(%s): must specify at least one source", archivePath)
	}

	// Create the operation
	op, err := b.createOperation("create_archive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"format":       format,
		"source_count": len(sources),
		"sources":      sources,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateArchive(%s): %w", archivePath, err)
	}

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
func (b *BatchImpl) Unarchive(archivePath, extractPath string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"extract_path": extractPath,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
func (b *BatchImpl) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"extract_path":  extractPath,
		"patterns":      patterns,
		"pattern_count": len(patterns),
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add UnarchiveWithPatterns(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// Run runs all operations in the batch using default options.
func (b *BatchImpl) Run() (interface{}, error) {
	// Use default pipeline options
	defaultOpts := map[string]interface{}{
		"restorable":        false,
		"max_backup_size_mb": 0,
	}
	return b.RunWithOptions(defaultOpts)
}

// RunWithOptions runs all operations in the batch with specified options.
func (b *BatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	startTime := time.Now()
	
	// Extract options and convert to core.PipelineOptions
	pipelineOpts := core.PipelineOptions{
		Restorable:      false,
		MaxBackupSizeMB: 10,
	}
	
	if optsMap, ok := opts.(map[string]interface{}); ok {
		if r, ok := optsMap["restorable"].(bool); ok {
			pipelineOpts.Restorable = r
		}
		if mb, ok := optsMap["max_backup_size_mb"].(int); ok {
			pipelineOpts.MaxBackupSizeMB = mb
		}
	}
	
	// Log the start of execution
	logInfo("executing batch", map[string]interface{}{
		"operation_count": len(b.operations),
		"restorable": pipelineOpts.Restorable,
		"max_backup_mb": pipelineOpts.MaxBackupSizeMB,
	})
	
	// If no operations, return successful empty result
	if len(b.operations) == 0 {
		duration := time.Since(startTime)
		logInfo("batch execution completed", map[string]interface{}{
			"success": true,
			"duration": duration,
			"operations_executed": 0,
		})
		
		batchResult := NewResult(true, b.operations, []interface{}{}, duration, nil)
		return batchResult, nil
	}
	
	// Create executor and pipeline using execution package
	logger := &simpleLogger{}
	executor := execution.NewExecutor(logger)
	
	// Create pipeline adapter
	pipeline := &pipelineAdapter{operations: b.operations}
	
	// Execute using the execution package
	coreResult := executor.RunWithOptions(b.ctx, pipeline, b.fs, pipelineOpts)
	
	duration := time.Since(startTime)
	
	// Convert core.Result back to our interface{} result
	var executionError error
	if !coreResult.Success && len(coreResult.Errors) > 0 {
		executionError = coreResult.Errors[0] // Take first error
	}
	
	// Extract restore operations 
	var restoreOps []interface{}
	if coreResult.RestoreOps != nil {
		restoreOps = coreResult.RestoreOps
	}
	
	logInfo("batch execution completed", map[string]interface{}{
		"success": coreResult.Success,
		"duration": duration,
		"operations_executed": len(coreResult.Operations),
		"restore_operations": len(restoreOps),
	})
	
	// Convert to batch result interface
	batchResult := NewResult(
		coreResult.Success,
		b.operations,
		restoreOps,
		duration,
		executionError,
	)
	
	return batchResult, nil
}

// RunRestorable runs all operations with backup enabled using the default 10MB budget.
func (b *BatchImpl) RunRestorable() (interface{}, error) {
	return b.RunRestorableWithBudget(10)
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget.
func (b *BatchImpl) RunRestorableWithBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"restorable":        true,
		"max_backup_size_mb": maxBackupMB,
	}
	return b.RunWithOptions(opts)
}

// Helper methods

// generateID creates a unique operation ID based on type and path.
func (b *BatchImpl) generateID(opType, path string) core.OperationID {
	b.idCounter++
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	return core.OperationID(fmt.Sprintf("batch_%d_%s_%s", b.idCounter, opType, cleanPath))
}

// createOperation is a helper method to create operations using the registry
func (b *BatchImpl) createOperation(opType, path string) (interface{}, error) {
	opID := b.generateID(opType, path)
	return b.registry.CreateOperation(opID, opType, path)
}

// setOperationDetails sets details on an operation through interface assertion
func (b *BatchImpl) setOperationDetails(op interface{}, details map[string]interface{}) error {
	type detailSetter interface {
		SetDescriptionDetail(key string, value interface{})
	}

	setter, ok := op.(detailSetter)
	if !ok {
		return fmt.Errorf("operation does not support setting details")
	}

	for key, value := range details {
		setter.SetDescriptionDetail(key, value)
	}

	return nil
}

// setOperationPaths sets paths on an operation through interface assertion
func (b *BatchImpl) setOperationPaths(op interface{}, src, dst string) error {
	type pathSetter interface {
		SetPaths(src, dst string)
	}

	setter, ok := op.(pathSetter)
	if !ok {
		return fmt.Errorf("operation does not support setting paths")
	}

	setter.SetPaths(src, dst)
	return nil
}

// TODO: Implement ensureParentDirectories when adding auto parent directory creation
// ensureParentDirectories analyzes a path and adds CreateDir operations for missing parent directories.
/*
func (b *BatchImpl) ensureParentDirectories(path string) []core.OperationID {
	// Clean and normalize the path
	cleanPath := filepath.Clean(path)
	parentDir := filepath.Dir(cleanPath)

	var dependencyIDs []core.OperationID

	// If parent is root or current directory, no parent needed
	if parentDir == "." || parentDir == "/" || parentDir == cleanPath {
		return dependencyIDs
	}

	// TODO: Check if parent directory exists or is projected to exist
	// For now, we'll create parent directories as needed

	// Recursively ensure parent's parents exist
	parentDeps := b.ensureParentDirectories(parentDir)
	dependencyIDs = append(dependencyIDs, parentDeps...)

	// Create operation for the parent directory
	parentOp, err := b.createOperation("create_directory", parentDir)
	if err != nil {
		return dependencyIDs
	}

	// Set directory mode
	_ = b.setOperationDetails(parentOp, map[string]interface{}{
		"mode": "0755",
	})

	// Add dependencies from parent's parents
	if depAdder, ok := parentOp.(interface{ AddDependency(core.OperationID) }); ok {
		for _, depID := range parentDeps {
			depAdder.AddDependency(depID)
		}
	}

	// Add to batch
	if err := b.add(parentOp); err == nil {
		// Get operation ID
		if idGetter, ok := parentOp.(interface{ ID() core.OperationID }); ok {
			dependencyIDs = append(dependencyIDs, idGetter.ID())
		}
	}

	return dependencyIDs
}
*/

// operationAdapter wraps interface{} operations to implement execution.OperationInterface
type operationAdapter struct {
	op interface{}
}

// newOperationAdapter creates a new operation adapter
func newOperationAdapter(op interface{}) *operationAdapter {
	return &operationAdapter{op: op}
}

func (oa *operationAdapter) ID() core.OperationID {
	if op, ok := oa.op.(interface{ ID() core.OperationID }); ok {
		return op.ID()
	}
	return core.OperationID("")
}

func (oa *operationAdapter) Describe() core.OperationDesc {
	if op, ok := oa.op.(interface{ Describe() core.OperationDesc }); ok {
		return op.Describe()
	}
	return core.OperationDesc{}
}

func (oa *operationAdapter) Dependencies() []core.OperationID {
	if op, ok := oa.op.(interface{ Dependencies() []core.OperationID }); ok {
		return op.Dependencies()
	}
	return []core.OperationID{}
}

func (oa *operationAdapter) Conflicts() []core.OperationID {
	if op, ok := oa.op.(interface{ Conflicts() []core.OperationID }); ok {
		return op.Conflicts()
	}
	return []core.OperationID{}
}

func (oa *operationAdapter) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Try ExecuteV2 first
	if op, ok := oa.op.(interface{ ExecuteV2(interface{}, *core.ExecutionContext, interface{}) error }); ok {
		return op.ExecuteV2(ctx, execCtx, fsys)
	}
	
	// Fallback to Execute if available
	if op, ok := oa.op.(interface{ Execute(context.Context, interface{}) error }); ok {
		if ctxTyped, ok := ctx.(context.Context); ok {
			return op.Execute(ctxTyped, fsys)
		}
	}
	
	return fmt.Errorf("operation does not implement ExecuteV2 or Execute methods")
}

func (oa *operationAdapter) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Try ValidateV2 first
	if op, ok := oa.op.(interface{ ValidateV2(interface{}, *core.ExecutionContext, interface{}) error }); ok {
		return op.ValidateV2(ctx, execCtx, fsys)
	}
	
	// Fallback to Validate if available
	if op, ok := oa.op.(interface{ Validate(context.Context, interface{}) error }); ok {
		if ctxTyped, ok := ctx.(context.Context); ok {
			return op.Validate(ctxTyped, fsys)
		}
	}
	
	return nil // No validation available
}

func (oa *operationAdapter) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	if op, ok := oa.op.(interface{ ReverseOps(context.Context, interface{}, *core.BackupBudget) ([]interface{}, *core.BackupData, error) }); ok {
		return op.ReverseOps(ctx, fsys, budget)
	}
	return nil, nil, nil
}

func (oa *operationAdapter) Rollback(ctx context.Context, fsys interface{}) error {
	if op, ok := oa.op.(interface{ Rollback(context.Context, interface{}) error }); ok {
		return op.Rollback(ctx, fsys)
	}
	return nil
}

func (oa *operationAdapter) GetItem() interface{} {
	if op, ok := oa.op.(interface{ GetItem() interface{} }); ok {
		return op.GetItem()
	}
	return nil
}

// pipelineAdapter adapts our operations to execution.PipelineInterface
type pipelineAdapter struct {
	operations []interface{}
}

func (pa *pipelineAdapter) Operations() []execution.OperationInterface {
	var result []execution.OperationInterface
	for _, op := range pa.operations {
		result = append(result, newOperationAdapter(op))
	}
	return result
}

func (pa *pipelineAdapter) Resolve() error {
	// TODO: Implement dependency resolution
	// For now, return no error
	return nil
}

func (pa *pipelineAdapter) Validate(ctx context.Context, fs interface{}) error {
	// TODO: Implement pipeline validation
	// For now, return no error
	return nil
}

// simpleLogger implements core.Logger for use with execution package
type simpleLogger struct{}

func (l *simpleLogger) Trace() core.LogEvent { return &simpleLogEvent{} }
func (l *simpleLogger) Debug() core.LogEvent { return &simpleLogEvent{} }
func (l *simpleLogger) Info() core.LogEvent  { return &simpleLogEvent{} }
func (l *simpleLogger) Warn() core.LogEvent  { return &simpleLogEvent{} }
func (l *simpleLogger) Error() core.LogEvent { return &simpleLogEvent{} }

// simpleLogEvent implements core.LogEvent
type simpleLogEvent struct{}

func (e *simpleLogEvent) Str(key, val string) core.LogEvent             { return e }
func (e *simpleLogEvent) Int(key string, val int) core.LogEvent         { return e }
func (e *simpleLogEvent) Bool(key string, val bool) core.LogEvent       { return e }
func (e *simpleLogEvent) Dur(key string, val interface{}) core.LogEvent { return e }
func (e *simpleLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *simpleLogEvent) Err(err error) core.LogEvent                   { return e }
func (e *simpleLogEvent) Float64(key string, val float64) core.LogEvent { return e }
func (e *simpleLogEvent) Msg(msg string)                                {}
