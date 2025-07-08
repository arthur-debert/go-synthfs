package batch

import (
	"context"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)



// BatchImpl represents a collection of operations that can be validated and executed as a unit.
// This implementation uses interface{} types to avoid circular dependencies.
type BatchImpl struct {
	operations    []interface{}
	fs            interface{} // Filesystem interface
	ctx           context.Context
	idCounter     int
	pathTracker   *pathStateTracker // Path state tracker
	registry      core.OperationFactory
	logger        core.Logger
	useSimpleBatch bool // Migration option: when true, use SimpleBatch behavior + prerequisite resolution
}

// BatchOptions provides configuration for batch creation
type BatchOptions struct {
	UseSimpleBatch bool // Enable new prerequisite-based design (default: false for backward compatibility)
}

// NewBatch creates a new operation batch with default options (backward compatible).
func NewBatch(fs interface{}, registry core.OperationFactory) Batch {
	return NewBatchWithOptions(fs, registry, BatchOptions{UseSimpleBatch: false})
}

// NewBatchWithOptions creates a new operation batch with specified options.
func NewBatchWithOptions(fs interface{}, registry core.OperationFactory, opts BatchOptions) Batch {
	return &BatchImpl{
		operations:    []interface{}{},
		fs:            fs,
		ctx:           context.Background(),
		idCounter:     0,
		pathTracker:   newPathStateTracker(fs),
		registry:      registry,
		logger:        nil, // Will be set by WithLogger method
		useSimpleBatch: opts.UseSimpleBatch,
	}
}

// NewBatchWithSimpleBatch creates a new operation batch with SimpleBatch behavior enabled.
// This disables automatic parent directory creation and relies on prerequisite resolution.
// 
// RECOMMENDED: This is now the preferred way to create batches as of Phase 6.
// This constructor explicitly enables the new behavior that relies on prerequisite resolution
// instead of hardcoded parent directory creation logic.
func NewBatchWithSimpleBatch(fs interface{}, registry core.OperationFactory) Batch {
	return &BatchImpl{
		operations:     []interface{}{},
		fs:             fs,
		ctx:            context.Background(),
		idCounter:      0,
		pathTracker:    nil, // No path tracker needed for SimpleBatch behavior
		registry:       registry,
		logger:         nil, // Will be set by WithLogger method
		useSimpleBatch: true, // Enable SimpleBatch behavior
	}
}

// NewBatchWithLegacyBehavior creates a new operation batch with legacy behavior enabled.
// This enables automatic parent directory creation and path tracking.
// 
// DEPRECATED: This constructor is provided for backward compatibility only.
// The legacy behavior with automatic parent directory creation and path tracking is deprecated.
// Please migrate to NewBatchWithSimpleBatch() or NewBatch() which use prerequisite resolution.
// This constructor will be removed in a future version.
func NewBatchWithLegacyBehavior(fs interface{}, registry core.OperationFactory) Batch {
	return &BatchImpl{
		operations:     []interface{}{},
		fs:             fs,
		ctx:            context.Background(),
		idCounter:      0,
		pathTracker:    newPathStateTracker(fs), // Enable path tracker for legacy behavior
		registry:       registry,
		logger:         nil, // Will be set by WithLogger method
		useSimpleBatch: false, // Disable SimpleBatch behavior for legacy support
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
	// Recreate pathTracker with new filesystem
	b.pathTracker = newPathStateTracker(fs)
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

// WithLogger sets the logger for the batch.
func (b *BatchImpl) WithLogger(logger core.Logger) Batch {
	b.logger = logger
	return b
}

// WithSimpleBatch sets the SimpleBatch behavior flag for migration.
// When true, the batch will use the new prerequisite-based design.
// When false (default), the batch uses the legacy behavior with automatic parent directory creation.
func (b *BatchImpl) WithSimpleBatch(useSimpleBatch bool) Batch {
	b.useSimpleBatch = useSimpleBatch
	// If switching to SimpleBatch, we don't need path tracking
	if useSimpleBatch {
		b.pathTracker = nil
	} else if b.pathTracker == nil {
		// If switching back to legacy, recreate path tracker
		b.pathTracker = newPathStateTracker(b.fs)
	}
	return b
}

// add adds an operation to the batch and validates it
func (b *BatchImpl) add(op interface{}) error {
	// Validate the operation first
	if err := b.validateOperation(op); err != nil {
		// For create operations, if the error is "file already exists" and the file
		// is scheduled for deletion, we should give a more specific error
		if b.pathTracker != nil && !b.useSimpleBatch {
			// Get operation description to check if it's a create operation
			if describer, ok := op.(interface{ Describe() core.OperationDesc }); ok {
				desc := describer.Describe()
				if strings.HasPrefix(desc.Type, "create") && b.pathTracker.isDeleted(desc.Path) {
					// Check if the error is about file existence
					if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "file already exists") {
						opID := ""
						if idGetter, ok := op.(interface{ ID() core.OperationID }); ok {
							opID = string(idGetter.ID())
						}
						return fmt.Errorf("validation error for operation %s (%s): path was scheduled for deletion", 
							opID, desc.Path)
					}
				}
			}
		}
		return err
	}

	// Check for conflicts with projected state (only in legacy mode)
	if !b.useSimpleBatch {
		if err := b.checkPathConflicts(op); err != nil {
			return err
		}

		// Update projected state
		if b.pathTracker != nil {
			if err := b.pathTracker.updateState(op); err != nil {
				return err
			}
		}

		// Auto-create parent directories if needed (legacy behavior)
		if err := b.autoCreateParentDirs(op); err != nil {
			return err
		}
	}

	b.operations = append(b.operations, op)
	return nil
}

// addWithoutAutoParent adds an operation without auto-creating parent directories
// This is used when creating parent directories to avoid infinite recursion
func (b *BatchImpl) addWithoutAutoParent(op interface{}) error {
	// Validate the operation first
	if err := b.validateOperation(op); err != nil {
		// For create operations, if the error is "file already exists" and the file
		// is scheduled for deletion, we should give a more specific error
		if b.pathTracker != nil {
			// Get operation description to check if it's a create operation
			if describer, ok := op.(interface{ Describe() core.OperationDesc }); ok {
				desc := describer.Describe()
				if strings.HasPrefix(desc.Type, "create") && b.pathTracker.isDeleted(desc.Path) {
					// Check if the error is about file existence
					if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "file already exists") {
						opID := ""
						if idGetter, ok := op.(interface{ ID() core.OperationID }); ok {
							opID = string(idGetter.ID())
						}
						return fmt.Errorf("validation error for operation %s (%s): path was scheduled for deletion", 
							opID, desc.Path)
					}
				}
			}
		}
		return err
	}

	// Check for conflicts with projected state
	if err := b.checkPathConflicts(op); err != nil {
		return err
	}

	// Update projected state
	if b.pathTracker != nil {
		if err := b.pathTracker.updateState(op); err != nil {
			return err
		}
	}

	b.operations = append(b.operations, op)
	return nil
}

// validateOperation validates an operation
func (b *BatchImpl) validateOperation(op interface{}) error {
	// Try to validate the operation
	// First check if it has a Validate method that accepts interface{}
	type validator interface {
		Validate(ctx context.Context, fsys interface{}) error
	}

	validated := false
	if v, ok := op.(validator); ok {
		if err := v.Validate(b.ctx, b.fs); err != nil {
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
			if err := v.ValidateV2(b.ctx, execCtx, b.fs); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkPathConflicts checks for conflicts with the projected state
func (b *BatchImpl) checkPathConflicts(op interface{}) error {
	if b.pathTracker == nil {
		return nil
	}

	// Get operation description
	type describer interface {
		Describe() core.OperationDesc
	}
	d, ok := op.(describer)
	if !ok {
		return nil
	}

	desc := d.Describe()
	path := desc.Path

	// Check for conflicts based on operation type
	switch desc.Type {
	case "create_file", "create_directory", "create_symlink", "create_archive":
		// Check if path already exists in projected state
		state, err := b.pathTracker.getState(path)
		if err == nil && state != nil && state.WillExist {
			// Special case: creating after deletion is OK
			if !b.pathTracker.isDeleted(path) {
				return fmt.Errorf("path %s conflicts with existing state", path)
			}
		}

	case "copy", "move":
		// Get destination path
		if details, ok := desc.Details["destination"]; ok {
			if dst, ok := details.(string); ok {
				// Check if destination already exists
				state, err := b.pathTracker.getState(dst)
				if err == nil && state != nil && state.WillExist {
					if !b.pathTracker.isDeleted(dst) {
						return fmt.Errorf("destination path %s conflicts with existing state", dst)
					}
				}
			}
		}

	case "delete":
		// Don't check here - let the PathStateTracker handle all delete validation
		// to ensure consistent error messages
	}

	return nil
}

// autoCreateParentDirs automatically creates parent directories if needed
func (b *BatchImpl) autoCreateParentDirs(op interface{}) error {
	// Get operation description
	type describer interface {
		Describe() core.OperationDesc
	}
	d, ok := op.(describer)
	if !ok {
		return nil
	}

	desc := d.Describe()
	
	// Only create parent dirs for operations that create new paths
	switch desc.Type {
	case "create_file", "create_directory", "create_symlink", "create_archive":
		parentDeps, err := ensureParentDirectories(b, desc.Path)
		if err != nil {
			return fmt.Errorf("failed to ensure parent directories: %w", err)
		}
		
		// Add dependencies to the operation
		if len(parentDeps) > 0 {
			if depAdder, ok := op.(interface{ AddDependency(core.OperationID) }); ok {
				for _, depID := range parentDeps {
					depAdder.AddDependency(depID)
				}
			}
		}
		
	case "copy", "move":
		// Also check destination path
		if details, ok := desc.Details["destination"]; ok {
			if dst, ok := details.(string); ok {
				parentDeps, err := ensureParentDirectories(b, dst)
				if err != nil {
					return fmt.Errorf("failed to ensure parent directories for destination: %w", err)
				}
				
				// Add dependencies to the operation
				if len(parentDeps) > 0 {
					if depAdder, ok := op.(interface{ AddDependency(core.OperationID) }); ok {
						for _, depID := range parentDeps {
							depAdder.AddDependency(depID)
						}
					}
				}
			}
		}
	}

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

	// Create and set the directory item for this operation
	dirItem := targets.NewDirectory(path).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateDir operation: %w", err)
	}

	// Set operation details
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

	// Create and set the file item for this operation
	fileItem := targets.NewFile(path).WithContent(content).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateFile operation: %w", err)
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
		"src": src,
		"dst": dst,
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

	// Compute checksum for source file (after validation passes)
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			// Set checksum on operation
			type checksumSetter interface {
				SetChecksum(path string, checksum *validation.ChecksumRecord)
			}
			if setter, ok := op.(checksumSetter); ok {
				setter.SetChecksum(src, checksum)
			}
			// Set source_checksum in details
			_ = b.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
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
		"src": src,
		"dst": dst,
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

	// Compute checksum for source file (after validation passes)
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			// Set checksum on operation
			type checksumSetter interface {
				SetChecksum(path string, checksum *validation.ChecksumRecord)
			}
			if setter, ok := op.(checksumSetter); ok {
				setter.SetChecksum(src, checksum)
			}
			// Set source_checksum in details
			_ = b.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
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

	// Create and set the symlink item for this operation
	symlinkItem := targets.NewSymlink(linkPath, target)
	if err := b.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateSymlink operation: %w", err)
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

	// Compute checksums for all source files
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
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
		_ = b.setOperationDetails(op, map[string]interface{}{
			"sources_checksummed": len(sources),
		})
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

	// Create and set the unarchive item for this operation
	unarchiveItem := targets.NewUnarchive(archivePath, extractPath)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for Unarchive operation: %w", err)
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

	// Create and set the unarchive item for this operation with patterns
	unarchiveItem := targets.NewUnarchive(archivePath, extractPath).WithPatterns(patterns...)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for UnarchiveWithPatterns operation: %w", err)
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
		"restorable":            false,
		"max_backup_size_mb":    0,
		"resolve_prerequisites": b.useSimpleBatch, // Enable prerequisite resolution for SimpleBatch behavior
	}
	return b.RunWithOptions(defaultOpts)
}

// RunWithOptions runs all operations in the batch with specified options.
func (b *BatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	startTime := time.Now()
	
	// Extract options and convert to core.PipelineOptions
	pipelineOpts := core.PipelineOptions{
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: b.useSimpleBatch, // Default based on batch mode
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
	if b.logger != nil {
		b.logger.Info().
			Int("operation_count", len(b.operations)).
			Bool("restorable", pipelineOpts.Restorable).
			Int("max_backup_mb", pipelineOpts.MaxBackupSizeMB).
			Bool("resolve_prerequisites", pipelineOpts.ResolvePrerequisites).
			Bool("use_simple_batch", b.useSimpleBatch).
			Msg("executing batch")
	}
	
	// If no operations, return successful empty result
	if len(b.operations) == 0 {
		duration := time.Since(startTime)
		if b.logger != nil {
			b.logger.Info().
				Bool("success", true).
				Dur("duration", duration).
				Int("operations_executed", 0).
				Msg("batch execution completed")
		}
		
		batchResult := NewResult(true, b.operations, []interface{}{}, duration, nil)
		return batchResult, nil
	}
	
	// Create executor and pipeline using execution package
	loggerToUse := b.logger
	if loggerToUse == nil {
		// Create a no-op logger if none provided
		loggerToUse = &noOpLogger{}
	}
	executor := execution.NewExecutor(loggerToUse)
	
	// Create pipeline adapter
	pipeline := &pipelineAdapter{operations: b.operations, registry: b.registry}
	
	// Create prerequisite resolver if prerequisite resolution is enabled
	var prereqResolver core.PrerequisiteResolver
	if pipelineOpts.ResolvePrerequisites {
		prereqResolver = execution.NewPrerequisiteResolver(b.registry, loggerToUse)
		if b.logger != nil {
			b.logger.Info().Msg("created prerequisite resolver with operation factory")
		}
	}
	
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
	
	if b.logger != nil {
		b.logger.Info().
			Bool("success", coreResult.Success).
			Dur("duration", duration).
			Int("operations_executed", len(coreResult.Operations)).
			Int("restore_operations", len(restoreOps)).
			Msg("batch execution completed")
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

// RunWithPrerequisites runs all operations with prerequisite resolution enabled.
// This method forces prerequisite resolution regardless of the batch mode.
func (b *BatchImpl) RunWithPrerequisites() (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true, // Force prerequisite resolution
		"restorable":            false,
		"max_backup_size_mb":    0,
	}
	return b.RunWithOptions(opts)
}

// RunWithPrerequisitesAndBudget runs all operations with prerequisite resolution and backup enabled.
// This method forces prerequisite resolution regardless of the batch mode.
func (b *BatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true, // Force prerequisite resolution
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
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


// operationAdapter wraps interface{} operations to implement execution.OperationInterface
type operationAdapter struct {
	op interface{}
}

// newOperationAdapter creates a new operation adapter
func newOperationAdapter(op interface{}) *operationAdapter {
	return &operationAdapter{op: op}
}

// GetOriginalOperation returns the wrapped operation
func (oa *operationAdapter) GetOriginalOperation() interface{} {
	return oa.op
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
	// Use reflection to call the method with the correct signature
	// OperationsPackageAdapter.ReverseOps expects (context.Context, FileSystem, *BackupBudget) returns ([]Operation, *BackupData, error)
	opValue := reflect.ValueOf(oa.op)
	reverseOpsMethod := opValue.MethodByName("ReverseOps")
	
	if !reverseOpsMethod.IsValid() {
		return nil, nil, nil
	}
	
	// Prepare arguments
	ctxValue := reflect.ValueOf(ctx)
	fsysValue := reflect.ValueOf(fsys)
	budgetValue := reflect.ValueOf(budget)
	
	// Call the method
	results := reverseOpsMethod.Call([]reflect.Value{ctxValue, fsysValue, budgetValue})
	
	if len(results) != 3 {
		return nil, nil, nil
	}
	
	// Extract results
	var ops []interface{}
	var backupData *core.BackupData
	var err error
	
	// Convert operations slice
	if !results[0].IsNil() {
		opsSlice := results[0].Interface()
		// Try to convert to []interface{}
		if opsReflect := reflect.ValueOf(opsSlice); opsReflect.Kind() == reflect.Slice {
			for i := 0; i < opsReflect.Len(); i++ {
				ops = append(ops, opsReflect.Index(i).Interface())
			}
		}
	}
	
	// Extract backup data
	if !results[1].IsNil() {
		backupData = results[1].Interface().(*core.BackupData)
	}
	
	// Extract error
	if !results[2].IsNil() {
		err = results[2].Interface().(error)
	}
	
	return ops, backupData, err
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

func (oa *operationAdapter) AddDependency(depID core.OperationID) {
	if op, ok := oa.op.(interface{ AddDependency(core.OperationID) }); ok {
		op.AddDependency(depID)
	}
}

func (oa *operationAdapter) SetDescriptionDetail(key string, value interface{}) {
	if op, ok := oa.op.(interface{ SetDescriptionDetail(string, interface{}) }); ok {
		op.SetDescriptionDetail(key, value)
	}
}

// pipelineAdapter adapts our operations to execution.PipelineInterface
type pipelineAdapter struct {
	operations []interface{}
	registry   core.OperationFactory
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

func (pa *pipelineAdapter) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	// Delegate to the execution package's pipeline implementation
	// Create a memory pipeline and add our operations
	pipeline := execution.NewMemPipeline(&noOpLogger{})
	
	// Add all operations to the pipeline
	for _, op := range pa.operations {
		if opInterface, ok := op.(execution.OperationInterface); ok {
			if err := pipeline.Add(opInterface); err != nil {
				return fmt.Errorf("failed to add operation to pipeline: %w", err)
			}
		} else {
			// Wrap the operation if it doesn't implement execution.OperationInterface
			adapter := newOperationAdapter(op)
			if err := pipeline.Add(adapter); err != nil {
				return fmt.Errorf("failed to add adapted operation to pipeline: %w", err)
			}
		}
	}
	
	// Call the pipeline's ResolvePrerequisites method
	return pipeline.ResolvePrerequisites(resolver, fs)
}

func (pa *pipelineAdapter) Validate(ctx context.Context, fs interface{}) error {
	// TODO: Implement pipeline validation
	// For now, return no error
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

