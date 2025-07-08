package batch

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

// SimpleBatch is a simplified batch implementation that relies on prerequisite resolution
// instead of hardcoded parent directory creation logic
type SimpleBatch struct {
	pipeline execution.Pipeline
	registry core.OperationFactory
	fs       interface{}
	ctx      context.Context
	logger   core.Logger
	idCounter int
}

// NewSimpleBatch creates a new simplified batch that uses prerequisite resolution
func NewSimpleBatch(fs interface{}, registry core.OperationFactory) Batch {
	return &SimpleBatch{
		pipeline:  execution.NewMemPipeline(nil),
		registry:  registry,
		fs:        fs,
		ctx:       context.Background(),
		logger:    nil,
		idCounter: 0,
	}
}

// Operations returns all operations currently in the batch.
func (sb *SimpleBatch) Operations() []interface{} {
	return sb.pipeline.Operations()
}

// WithFileSystem sets the filesystem for the batch operations.
func (sb *SimpleBatch) WithFileSystem(fs interface{}) Batch {
	sb.fs = fs
	return sb
}

// WithContext sets the context for the batch operations.
func (sb *SimpleBatch) WithContext(ctx context.Context) Batch {
	sb.ctx = ctx
	return sb
}

// WithRegistry sets a custom operation registry for the batch.
func (sb *SimpleBatch) WithRegistry(registry core.OperationFactory) Batch {
	sb.registry = registry
	return sb
}

// WithLogger sets the logger for the batch.
func (sb *SimpleBatch) WithLogger(logger core.Logger) Batch {
	sb.logger = logger
	// Also update the pipeline logger
	sb.pipeline = execution.NewMemPipeline(logger)
	return sb
}

// CreateDir adds a directory creation operation to the batch.
func (sb *SimpleBatch) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0755) // Default directory mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation using the registry
	op, err := sb.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	// Create and set the directory item for this operation
	dirItem := targets.NewDirectory(path).WithMode(fileMode)
	if err := sb.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateDir operation: %w", err)
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"mode": fileMode.String(),
	}); err != nil {
		return nil, err
	}

	// Add to pipeline (no auto-parent creation - let prerequisites handle it)
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateDir operation to pipeline: %w", err)
	}

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
func (sb *SimpleBatch) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0644) // Default file mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation
	op, err := sb.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	// Create and set the file item for this operation
	fileItem := targets.NewFile(path).WithContent(content).WithMode(fileMode)
	if err := sb.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateFile operation: %w", err)
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"content_length": len(content),
		"mode":           fileMode.String(),
		"content":        content,
	}); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateFile operation to pipeline: %w", err)
	}

	return op, nil
}

// Copy adds a copy operation to the batch.
func (sb *SimpleBatch) Copy(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src": src,
		"dst": dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := sb.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add Copy operation to pipeline: %w", err)
	}

	return op, nil
}

// Move adds a move operation to the batch.
func (sb *SimpleBatch) Move(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src": src,
		"dst": dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := sb.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add Move operation to pipeline: %w", err)
	}

	return op, nil
}

// Delete adds a delete operation to the batch.
func (sb *SimpleBatch) Delete(path string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete operation to pipeline: %w", err)
	}

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (sb *SimpleBatch) CreateSymlink(target, linkPath string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	// Create and set the symlink item for this operation
	symlinkItem := targets.NewSymlink(linkPath, target)
	if err := sb.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateSymlink operation: %w", err)
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"target": target,
	}); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateSymlink operation to pipeline: %w", err)
	}

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
func (sb *SimpleBatch) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
	// Validate inputs
	if len(sources) == 0 {
		return nil, fmt.Errorf("validation failed for CreateArchive(%s): must specify at least one source", archivePath)
	}

	// Create the operation
	op, err := sb.createOperation("create_archive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"format":       format,
		"source_count": len(sources),
		"sources":      sources,
	}); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateArchive operation to pipeline: %w", err)
	}

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
func (sb *SimpleBatch) Unarchive(archivePath, extractPath string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Create and set the unarchive item for this operation
	unarchiveItem := targets.NewUnarchive(archivePath, extractPath)
	if err := sb.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for Unarchive operation: %w", err)
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"extract_path": extractPath,
	}); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add Unarchive operation to pipeline: %w", err)
	}

	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
func (sb *SimpleBatch) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Create and set the unarchive item for this operation with patterns
	unarchiveItem := targets.NewUnarchive(archivePath, extractPath).WithPatterns(patterns...)
	if err := sb.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for UnarchiveWithPatterns operation: %w", err)
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"extract_path":  extractPath,
		"patterns":      patterns,
		"pattern_count": len(patterns),
	}); err != nil {
		return nil, err
	}

	// Add to pipeline
	if err := sb.pipeline.Add(op); err != nil {
		return nil, fmt.Errorf("failed to add UnarchiveWithPatterns operation to pipeline: %w", err)
	}

	return op, nil
}

// Run runs all operations in the batch using default options with prerequisite resolution enabled.
func (sb *SimpleBatch) Run() (interface{}, error) {
	defaultOpts := map[string]interface{}{
		"restorable":             false,
		"max_backup_size_mb":     0,
		"resolve_prerequisites":  true,
	}
	return sb.RunWithOptions(defaultOpts)
}

// RunWithOptions runs all operations in the batch with specified options.
func (sb *SimpleBatch) RunWithOptions(opts interface{}) (interface{}, error) {
	startTime := time.Now()
	
	// Extract options and convert to core.PipelineOptions
	pipelineOpts := core.PipelineOptions{
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: true, // Enable by default for SimpleBatch
	}
	
	if optsMap, ok := opts.(map[string]interface{}); ok {
		if r, ok := optsMap["restorable"].(bool); ok {
			pipelineOpts.Restorable = r
		}
		if mb, ok := optsMap["max_backup_size_mb"].(int); ok {
			pipelineOpts.MaxBackupSizeMB = mb
		}
		if rp, ok := optsMap["resolve_prerequisites"].(bool); ok {
			pipelineOpts.ResolvePrerequisites = rp
		}
	}
	
	// Log the start of execution
	if sb.logger != nil {
		sb.logger.Info().
			Int("operation_count", len(sb.Operations())).
			Bool("restorable", pipelineOpts.Restorable).
			Bool("resolve_prerequisites", pipelineOpts.ResolvePrerequisites).
			Int("max_backup_mb", pipelineOpts.MaxBackupSizeMB).
			Msg("executing simple batch")
	}
	
	// If no operations, return successful empty result
	operations := sb.Operations()
	if len(operations) == 0 {
		duration := time.Since(startTime)
		if sb.logger != nil {
			sb.logger.Info().
				Bool("success", true).
				Dur("duration", duration).
				Int("operations_executed", 0).
				Msg("simple batch execution completed")
		}
		
		return NewResult(true, operations, []interface{}{}, duration, nil), nil
	}
	
	// Create executor and run with prerequisite resolution
	loggerToUse := sb.logger
	if loggerToUse == nil {
		loggerToUse = &noOpLogger{}
	}
	executor := execution.NewExecutor(loggerToUse)
	
	// Create prerequisite resolver
	resolver := execution.NewPrerequisiteResolver(sb.registry, loggerToUse)
	
	// Execute using the execution package with prerequisite resolution
	coreResult := executor.RunWithOptionsAndResolver(sb.ctx, sb.pipeline, sb.fs, pipelineOpts, resolver)
	
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
	
	if sb.logger != nil {
		sb.logger.Info().
			Bool("success", coreResult.Success).
			Dur("duration", duration).
			Int("operations_executed", len(coreResult.Operations)).
			Int("restore_operations", len(restoreOps)).
			Msg("simple batch execution completed")
	}
	
	// Convert execution package results to interface{} slice
	var operationResults []interface{}
	for _, opResult := range coreResult.Operations {
		operationResults = append(operationResults, opResult)
	}
	
	// Convert to batch result interface
	batchResult := NewResultWithBudgetAndRollback(
		coreResult.Success,
		operationResults,
		restoreOps,
		duration,
		executionError,
		coreResult.Budget,
		coreResult.Rollback,
	)
	
	return batchResult, nil
}

// RunRestorable runs all operations with backup enabled using the default 10MB budget.
func (sb *SimpleBatch) RunRestorable() (interface{}, error) {
	return sb.RunRestorableWithBudget(10)
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget.
func (sb *SimpleBatch) RunRestorableWithBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"restorable":             true,
		"max_backup_size_mb":     maxBackupMB,
		"resolve_prerequisites":  true,
	}
	return sb.RunWithOptions(opts)
}

// Helper methods

// generateID creates a unique operation ID based on type and path.
func (sb *SimpleBatch) generateID(opType, path string) core.OperationID {
	sb.idCounter++
	cleanPath := path
	// Replace path separators to avoid issues in IDs
	cleanPath = fmt.Sprintf("simple_%d_%s_%s", sb.idCounter, opType, cleanPath)
	return core.OperationID(cleanPath)
}

// createOperation is a helper method to create operations using the registry
func (sb *SimpleBatch) createOperation(opType, path string) (interface{}, error) {
	opID := sb.generateID(opType, path)
	return sb.registry.CreateOperation(opID, opType, path)
}

// setOperationDetails sets details on an operation through interface assertion
func (sb *SimpleBatch) setOperationDetails(op interface{}, details map[string]interface{}) error {
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
func (sb *SimpleBatch) setOperationPaths(op interface{}, src, dst string) error {
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

// noOpLogger implements core.Logger for when no logger is provided
type noOpLogger struct{}

func (l *noOpLogger) Trace() core.LogEvent { return &noOpLogEvent{} }
func (l *noOpLogger) Debug() core.LogEvent { return &noOpLogEvent{} }
func (l *noOpLogger) Info() core.LogEvent  { return &noOpLogEvent{} }
func (l *noOpLogger) Warn() core.LogEvent  { return &noOpLogEvent{} }
func (l *noOpLogger) Error() core.LogEvent { return &noOpLogEvent{} }

// noOpLogEvent implements core.LogEvent with no-op methods
type noOpLogEvent struct{}

func (e *noOpLogEvent) Str(key, val string) core.LogEvent             { return e }
func (e *noOpLogEvent) Int(key string, val int) core.LogEvent         { return e }
func (e *noOpLogEvent) Bool(key string, val bool) core.LogEvent       { return e }
func (e *noOpLogEvent) Dur(key string, val interface{}) core.LogEvent { return e }
func (e *noOpLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *noOpLogEvent) Err(err error) core.LogEvent                   { return e }
func (e *noOpLogEvent) Float64(key string, val float64) core.LogEvent { return e }
func (e *noOpLogEvent) Msg(msg string)                                {}