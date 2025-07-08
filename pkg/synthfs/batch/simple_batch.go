package batch

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// SimpleBatchImpl is a simplified batch implementation that doesn't handle parent directory creation
// automatically. It relies on prerequisite resolution in the pipeline for dependency management.
type SimpleBatchImpl struct {
	operations []interface{}
	fs         interface{} // Filesystem interface
	ctx        context.Context
	idCounter  int
	registry   core.OperationFactory
	logger     core.Logger
}

// NewSimpleBatch creates a new simple operation batch.
func NewSimpleBatch(fs interface{}, registry core.OperationFactory) Batch {
	return &SimpleBatchImpl{
		operations: []interface{}{},
		fs:         fs,
		ctx:        context.Background(),
		idCounter:  0,
		registry:   registry,
		logger:     nil, // Will be set by WithLogger method
	}
}

// Operations returns all operations currently in the batch.
func (sb *SimpleBatchImpl) Operations() []interface{} {
	// Return a copy to prevent external modification
	opsCopy := make([]interface{}, len(sb.operations))
	copy(opsCopy, sb.operations)
	return opsCopy
}

// WithFileSystem sets the filesystem for the batch operations.
func (sb *SimpleBatchImpl) WithFileSystem(fs interface{}) Batch {
	sb.fs = fs
	return sb
}

// WithContext sets the context for the batch operations.
func (sb *SimpleBatchImpl) WithContext(ctx context.Context) Batch {
	sb.ctx = ctx
	return sb
}

// WithRegistry sets a custom operation registry for the batch.
func (sb *SimpleBatchImpl) WithRegistry(registry core.OperationFactory) Batch {
	sb.registry = registry
	return sb
}

// WithLogger sets the logger for the batch.
func (sb *SimpleBatchImpl) WithLogger(logger core.Logger) Batch {
	sb.logger = logger
	return sb
}

// add adds an operation to the batch with basic validation
func (sb *SimpleBatchImpl) add(op interface{}) error {
	// Basic validation - no path conflict checking or parent directory creation
	if err := sb.validateOperation(op); err != nil {
		return err
	}

	sb.operations = append(sb.operations, op)
	return nil
}

// validateOperation validates an operation
func (sb *SimpleBatchImpl) validateOperation(op interface{}) error {
	// Try to validate the operation
	// First check if it has a Validate method that accepts interface{}
	type validator interface {
		Validate(ctx context.Context, fsys interface{}) error
	}

	validated := false
	if v, ok := op.(validator); ok {
		if err := v.Validate(sb.ctx, sb.fs); err != nil {
			return err
		}
		validated = true
	}

	// If not validated yet, try ValidateV2 which operations should have
	if !validated {
		type validatorV2 interface {
			ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error
		}

		if v, ok := op.(validatorV2); ok {
			// Create a minimal ExecutionContext for validation
			execCtx := &core.ExecutionContext{}
			if err := v.ValidateV2(sb.ctx, execCtx, sb.fs); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateDir adds a directory creation operation to the batch.
func (sb *SimpleBatchImpl) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
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

	// Add to batch (simple validation only)
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
func (sb *SimpleBatchImpl) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
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

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	return op, nil
}

// Copy adds a copy operation to the batch.
func (sb *SimpleBatchImpl) Copy(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := sb.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	// Compute checksum for source file (after validation passes)
	if fs, ok := sb.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			// Set checksum on operation
			type checksumSetter interface {
				SetChecksum(path string, checksum *validation.ChecksumRecord)
			}
			if setter, ok := op.(checksumSetter); ok {
				setter.SetChecksum(src, checksum)
			}
			// Set source_checksum in details
			_ = sb.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
	}

	return op, nil
}

// Move adds a move operation to the batch.
func (sb *SimpleBatchImpl) Move(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := sb.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := sb.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	// Compute checksum for source file (after validation passes)
	if fs, ok := sb.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			// Set checksum on operation
			type checksumSetter interface {
				SetChecksum(path string, checksum *validation.ChecksumRecord)
			}
			if setter, ok := op.(checksumSetter); ok {
				setter.SetChecksum(src, checksum)
			}
			// Set source_checksum in details
			_ = sb.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
	}

	return op, nil
}

// Delete adds a delete operation to the batch.
func (sb *SimpleBatchImpl) Delete(path string) (interface{}, error) {
	// Create the operation
	op, err := sb.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete(%s): %w", path, err)
	}

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (sb *SimpleBatchImpl) CreateSymlink(target, linkPath string) (interface{}, error) {
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

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
func (sb *SimpleBatchImpl) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
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

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateArchive(%s): %w", archivePath, err)
	}

	// Compute checksums for all source files
	if fs, ok := sb.fs.(filesystem.FullFileSystem); ok {
		for _, source := range sources {
			if checksum, err := validation.ComputeFileChecksum(fs, source); err == nil && checksum != nil {
				// Set checksum on operation
				type checksumSetter interface {
					SetChecksum(path string, checksum *validation.ChecksumRecord)
				}
				if setter, ok := op.(checksumSetter); ok {
					setter.SetChecksum(source, checksum)
				}
			}
		}
		// Set sources_checksummed in details
		_ = sb.setOperationDetails(op, map[string]interface{}{
			"sources_checksummed": len(sources),
		})
	}

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
func (sb *SimpleBatchImpl) Unarchive(archivePath, extractPath string) (interface{}, error) {
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

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
func (sb *SimpleBatchImpl) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
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

	// Add to batch
	if err := sb.add(op); err != nil {
		return nil, fmt.Errorf("failed to add UnarchiveWithPatterns(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// Run runs all operations in the batch with prerequisite resolution enabled by default.
func (sb *SimpleBatchImpl) Run() (interface{}, error) {
	// SimpleBatch enables prerequisite resolution by default
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            false,
		"max_backup_size_mb":    0,
	}
	return sb.RunWithOptions(opts)
}

// RunWithOptions runs all operations in the batch with specified options.
func (sb *SimpleBatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	startTime := time.Now()

	// Extract options and convert to core.PipelineOptions
	pipelineOpts := core.PipelineOptions{
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: false,
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
			Int("operation_count", len(sb.operations)).
			Bool("restorable", pipelineOpts.Restorable).
			Int("max_backup_mb", pipelineOpts.MaxBackupSizeMB).
			Bool("resolve_prerequisites", pipelineOpts.ResolvePrerequisites).
			Msg("executing simple batch")
	}

	// If no operations, return successful empty result
	if len(sb.operations) == 0 {
		duration := time.Since(startTime)
		if sb.logger != nil {
			sb.logger.Info().
				Bool("success", true).
				Dur("duration", duration).
				Int("operations_executed", 0).
				Msg("simple batch execution completed")
		}

		batchResult := NewResult(true, sb.operations, []interface{}{}, duration, nil)
		return batchResult, nil
	}

	// Create executor and pipeline using execution package
	loggerToUse := sb.logger
	if loggerToUse == nil {
		// Create a no-op logger if none provided
		loggerToUse = &noOpLogger{}
	}
	executor := execution.NewExecutor(loggerToUse)

	// Create pipeline adapter
	pipeline := &simplePipelineAdapter{operations: sb.operations, registry: sb.registry}

	// Create prerequisite resolver if prerequisite resolution is enabled
	var prereqResolver core.PrerequisiteResolver
	if pipelineOpts.ResolvePrerequisites {
		prereqResolver = execution.NewPrerequisiteResolver(sb.registry, loggerToUse)
		if sb.logger != nil {
			sb.logger.Info().Msg("created prerequisite resolver with operation factory")
		}
	}

	// Execute using the execution package with prerequisite resolver
	coreResult := executor.RunWithOptionsAndResolver(sb.ctx, pipeline, sb.fs, pipelineOpts, prereqResolver)

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
func (sb *SimpleBatchImpl) RunRestorable() (interface{}, error) {
	return sb.RunRestorableWithBudget(10)
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget.
func (sb *SimpleBatchImpl) RunRestorableWithBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true, // SimpleBatch enables prerequisites by default
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
	}
	return sb.RunWithOptions(opts)
}

// RunWithPrerequisites runs all operations with prerequisite resolution enabled.
func (sb *SimpleBatchImpl) RunWithPrerequisites() (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            false,
		"max_backup_size_mb":    0,
	}
	return sb.RunWithOptions(opts)
}

// RunWithPrerequisitesAndBudget runs all operations with prerequisite resolution and backup enabled.
func (sb *SimpleBatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
	}
	return sb.RunWithOptions(opts)
}

// Helper methods

// generateID creates a unique operation ID based on type and path.
func (sb *SimpleBatchImpl) generateID(opType, path string) core.OperationID {
	sb.idCounter++
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	return core.OperationID(fmt.Sprintf("simple_batch_%d_%s_%s", sb.idCounter, opType, cleanPath))
}

// createOperation is a helper method to create operations using the registry
func (sb *SimpleBatchImpl) createOperation(opType, path string) (interface{}, error) {
	opID := sb.generateID(opType, path)
	return sb.registry.CreateOperation(opID, opType, path)
}

// setOperationDetails sets details on an operation through interface assertion
func (sb *SimpleBatchImpl) setOperationDetails(op interface{}, details map[string]interface{}) error {
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
func (sb *SimpleBatchImpl) setOperationPaths(op interface{}, src, dst string) error {
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

// simplePipelineAdapter adapts our operations to execution.PipelineInterface
type simplePipelineAdapter struct {
	operations []interface{}
	registry   core.OperationFactory
}

func (spa *simplePipelineAdapter) Operations() []execution.OperationInterface {
	// Convert operations to execution.OperationInterface
	var ops []interface{}
	for _, op := range spa.operations {
		ops = append(ops, op)
	}

	result := make([]execution.OperationInterface, len(ops))
	for i, op := range ops {
		if execOp, ok := op.(execution.OperationInterface); ok {
			result[i] = execOp
		} else {
			// Wrap the operation if it doesn't implement execution.OperationInterface
			result[i] = newOperationAdapter(op)
		}
	}
	return result
}

func (spa *simplePipelineAdapter) Resolve() error {
	// Create a memory pipeline to handle dependency resolution
	pipeline := execution.NewMemPipeline(&noOpLogger{})
	
	// Add operations to pipeline
	ops := spa.Operations()
	var interfaceOps []interface{}
	for _, op := range ops {
		interfaceOps = append(interfaceOps, op)
	}
	
	if err := pipeline.Add(interfaceOps...); err != nil {
		return fmt.Errorf("failed to add operations to pipeline: %w", err)
	}
	
	return pipeline.Resolve()
}

func (spa *simplePipelineAdapter) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	// Create a memory pipeline to handle prerequisite resolution
	pipeline := execution.NewMemPipeline(&noOpLogger{})
	
	// Add operations to pipeline
	ops := spa.Operations()
	var interfaceOps []interface{}
	for _, op := range ops {
		interfaceOps = append(interfaceOps, op)
	}
	
	if err := pipeline.Add(interfaceOps...); err != nil {
		return fmt.Errorf("failed to add operations to pipeline: %w", err)
	}
	
	if err := pipeline.ResolvePrerequisites(resolver, fs); err != nil {
		return fmt.Errorf("failed to resolve prerequisites: %w", err)
	}
	
	// Update our operations with resolved operations
	resolvedOps := pipeline.Operations()
	spa.operations = resolvedOps
	
	return nil
}

func (spa *simplePipelineAdapter) Validate(ctx context.Context, fs interface{}) error {
	// Create a memory pipeline to handle validation
	pipeline := execution.NewMemPipeline(&noOpLogger{})
	
	// Add operations to pipeline
	ops := spa.Operations()
	var interfaceOps []interface{}
	for _, op := range ops {
		interfaceOps = append(interfaceOps, op)
	}
	
	if err := pipeline.Add(interfaceOps...); err != nil {
		return fmt.Errorf("failed to add operations to pipeline: %w", err)
	}
	
	return pipeline.Validate(ctx, fs)
}

// noOpLogger is a no-op implementation of core.Logger
type noOpLogger struct{}

func (l *noOpLogger) Info() core.LogEvent  { return &noOpLogEvent{} }
func (l *noOpLogger) Debug() core.LogEvent { return &noOpLogEvent{} }
func (l *noOpLogger) Warn() core.LogEvent  { return &noOpLogEvent{} }
func (l *noOpLogger) Error() core.LogEvent { return &noOpLogEvent{} }
func (l *noOpLogger) Trace() core.LogEvent { return &noOpLogEvent{} }

type noOpLogEvent struct{}

func (e *noOpLogEvent) Str(key, val string) core.LogEvent                   { return e }
func (e *noOpLogEvent) Int(key string, val int) core.LogEvent               { return e }
func (e *noOpLogEvent) Err(err error) core.LogEvent                         { return e }
func (e *noOpLogEvent) Float64(key string, val float64) core.LogEvent       { return e }
func (e *noOpLogEvent) Bool(key string, val bool) core.LogEvent             { return e }
func (e *noOpLogEvent) Dur(key string, val interface{}) core.LogEvent       { return e }
func (e *noOpLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *noOpLogEvent) Msg(msg string)                                      {}