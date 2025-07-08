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

// SimpleBatchImpl represents a simplified batch implementation that doesn't handle prerequisites
// This implementation focuses on creating operations without hardcoded parent directory logic
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
		logger:     nil,
	}
}

// Operations returns all operations currently in the batch.
func (b *SimpleBatchImpl) Operations() []interface{} {
	// Return a copy to prevent external modification
	opsCopy := make([]interface{}, len(b.operations))
	copy(opsCopy, b.operations)
	return opsCopy
}

// WithFileSystem sets the filesystem for the batch operations.
func (b *SimpleBatchImpl) WithFileSystem(fs interface{}) Batch {
	b.fs = fs
	return b
}

// WithContext sets the context for the batch operations.
func (b *SimpleBatchImpl) WithContext(ctx context.Context) Batch {
	b.ctx = ctx
	return b
}

// WithRegistry sets a custom operation registry for the batch.
func (b *SimpleBatchImpl) WithRegistry(registry core.OperationFactory) Batch {
	b.registry = registry
	return b
}

// WithLogger sets the logger for the batch.
func (b *SimpleBatchImpl) WithLogger(logger core.Logger) Batch {
	b.logger = logger
	return b
}

// add adds an operation to the batch with minimal validation
func (b *SimpleBatchImpl) add(op interface{}) error {
	// Basic validation only - no hardcoded parent directory logic
	if err := b.validateOperation(op); err != nil {
		return err
	}

	b.operations = append(b.operations, op)
	return nil
}

// validateOperation performs basic validation without complex state tracking
func (b *SimpleBatchImpl) validateOperation(op interface{}) error {
	// Try to validate the operation
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

// CreateDir adds a directory creation operation to the batch.
func (b *SimpleBatchImpl) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
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

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
func (b *SimpleBatchImpl) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
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

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	return op, nil
}

// Copy adds a copy operation to the batch.
func (b *SimpleBatchImpl) Copy(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	return op, nil
}

// Move adds a move operation to the batch.
func (b *SimpleBatchImpl) Move(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	return op, nil
}

// Delete adds a delete operation to the batch.
func (b *SimpleBatchImpl) Delete(path string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete(%s): %w", path, err)
	}

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (b *SimpleBatchImpl) CreateSymlink(target, linkPath string) (interface{}, error) {
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

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
func (b *SimpleBatchImpl) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
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

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateArchive(%s): %w", archivePath, err)
	}

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
func (b *SimpleBatchImpl) Unarchive(archivePath, extractPath string) (interface{}, error) {
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

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
func (b *SimpleBatchImpl) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
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

	// Add to batch (no auto-parent creation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add UnarchiveWithPatterns(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// Run runs all operations in the batch using default options.
func (b *SimpleBatchImpl) Run() (interface{}, error) {
	// Use default pipeline options with prerequisite resolution enabled
	defaultOpts := map[string]interface{}{
		"restorable":            false,
		"max_backup_size_mb":    0,
		"resolve_prerequisites": true, // Enable prerequisite resolution by default
	}
	return b.RunWithOptions(defaultOpts)
}

// RunWithOptions runs all operations in the batch with specified options.
func (b *SimpleBatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	startTime := time.Now()

	// Extract options and convert to core.PipelineOptions
	pipelineOpts := core.PipelineOptions{
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: true, // Default to true for SimpleBatch
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
			Msg("executing simple batch")
	}

	// If no operations, return successful empty result
	if len(b.operations) == 0 {
		duration := time.Since(startTime)
		if b.logger != nil {
			b.logger.Info().
				Bool("success", true).
				Dur("duration", duration).
				Int("operations_executed", 0).
				Msg("simple batch execution completed")
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
	pipeline := &simplePipelineAdapter{operations: b.operations, registry: b.registry}

	// Create prerequisite resolver for prerequisite resolution
	var prereqResolver core.PrerequisiteResolver
	if pipelineOpts.ResolvePrerequisites {
		prereqResolver = execution.NewPrerequisiteResolver(b.registry, loggerToUse)
		if b.logger != nil {
			b.logger.Info().Msg("created prerequisite resolver for simple batch")
		}
	}

	// Execute using the execution package with prerequisite resolver
	coreResult := executor.RunWithOptionsAndResolver(b.ctx, pipeline, b.fs, pipelineOpts, prereqResolver)

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
func (b *SimpleBatchImpl) RunRestorable() (interface{}, error) {
	return b.RunRestorableWithBudget(10)
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget.
func (b *SimpleBatchImpl) RunRestorableWithBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
		"resolve_prerequisites": true,
	}
	return b.RunWithOptions(opts)
}

// RunWithPrerequisites runs all operations with prerequisite resolution enabled.
func (b *SimpleBatchImpl) RunWithPrerequisites() (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            false,
		"max_backup_size_mb":    0,
	}
	return b.RunWithOptions(opts)
}

// RunWithPrerequisitesAndBudget runs all operations with prerequisite resolution and backup enabled.
func (b *SimpleBatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
	}
	return b.RunWithOptions(opts)
}

// Helper methods

// generateID creates a unique operation ID based on type and path.
func (b *SimpleBatchImpl) generateID(opType, path string) core.OperationID {
	b.idCounter++
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	cleanPath = strings.ReplaceAll(cleanPath, ":", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "?", "_")
	return core.OperationID(fmt.Sprintf("simple_%d_%s_%s", b.idCounter, opType, cleanPath))
}

// createOperation is a helper method to create operations using the registry
func (b *SimpleBatchImpl) createOperation(opType, path string) (interface{}, error) {
	opID := b.generateID(opType, path)
	return b.registry.CreateOperation(opID, opType, path)
}

// setOperationDetails sets details on an operation through interface assertion
func (b *SimpleBatchImpl) setOperationDetails(op interface{}, details map[string]interface{}) error {
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
func (b *SimpleBatchImpl) setOperationPaths(op interface{}, src, dst string) error {
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

// simplePipelineAdapter adapts the operations for the execution package
type simplePipelineAdapter struct {
	operations []interface{}
	registry   core.OperationFactory
}

func (spa *simplePipelineAdapter) Operations() []execution.OperationInterface {
	// Convert operations to the format expected by execution package
	var result []execution.OperationInterface
	for _, op := range spa.operations {
		// Create adapter for each operation
		adapter := &simpleOperationAdapter{op: op}
		result = append(result, adapter)
	}
	return result
}

func (spa *simplePipelineAdapter) Resolve() error {
	// Simple implementation - no complex dependency resolution needed
	return nil
}

func (spa *simplePipelineAdapter) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	// This will be handled by the execution package's pipeline
	return nil
}

func (spa *simplePipelineAdapter) Validate(ctx context.Context, fs interface{}) error {
	// Basic validation - all operations should already be validated
	return nil
}

// simpleOperationAdapter wraps interface{} operations to implement execution.OperationInterface
type simpleOperationAdapter struct {
	op interface{}
}

// Ensure simpleOperationAdapter implements execution.OperationInterface
var _ execution.OperationInterface = (*simpleOperationAdapter)(nil)

func (soa *simpleOperationAdapter) GetOriginalOperation() interface{} {
	return soa.op
}

func (soa *simpleOperationAdapter) ID() core.OperationID {
	if op, ok := soa.op.(interface{ ID() core.OperationID }); ok {
		return op.ID()
	}
	return core.OperationID("")
}

func (soa *simpleOperationAdapter) Describe() core.OperationDesc {
	if op, ok := soa.op.(interface{ Describe() core.OperationDesc }); ok {
		return op.Describe()
	}
	return core.OperationDesc{}
}

func (soa *simpleOperationAdapter) Dependencies() []core.OperationID {
	if op, ok := soa.op.(interface{ Dependencies() []core.OperationID }); ok {
		return op.Dependencies()
	}
	return []core.OperationID{}
}

func (soa *simpleOperationAdapter) Conflicts() []core.OperationID {
	if op, ok := soa.op.(interface{ Conflicts() []core.OperationID }); ok {
		return op.Conflicts()
	}
	return []core.OperationID{}
}

func (soa *simpleOperationAdapter) AddDependency(depID core.OperationID) {
	if op, ok := soa.op.(interface{ AddDependency(core.OperationID) }); ok {
		op.AddDependency(depID)
	}
}

func (soa *simpleOperationAdapter) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	if op, ok := soa.op.(interface{ ExecuteV2(interface{}, *core.ExecutionContext, interface{}) error }); ok {
		return op.ExecuteV2(ctx, execCtx, fsys)
	}

	// Fallback to Execute if available
	if op, ok := soa.op.(interface{ Execute(context.Context, interface{}) error }); ok {
		if ctxTyped, ok := ctx.(context.Context); ok {
			return op.Execute(ctxTyped, fsys)
		}
	}

	return fmt.Errorf("operation does not implement ExecuteV2 or Execute methods")
}

func (soa *simpleOperationAdapter) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	if op, ok := soa.op.(interface{ ValidateV2(interface{}, *core.ExecutionContext, interface{}) error }); ok {
		return op.ValidateV2(ctx, execCtx, fsys)
	}

	// Fallback to Validate if available
	if op, ok := soa.op.(interface{ Validate(context.Context, interface{}) error }); ok {
		if ctxTyped, ok := ctx.(context.Context); ok {
			return op.Validate(ctxTyped, fsys)
		}
	}

	return nil // No validation available
}

func (soa *simpleOperationAdapter) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	// Use reflection-free approach
	if op, ok := soa.op.(interface{ ReverseOps(context.Context, interface{}, *core.BackupBudget) ([]interface{}, *core.BackupData, error) }); ok {
		return op.ReverseOps(ctx, fsys, budget)
	}
	return nil, nil, nil
}

func (soa *simpleOperationAdapter) Rollback(ctx context.Context, fsys interface{}) error {
	if op, ok := soa.op.(interface{ Rollback(context.Context, interface{}) error }); ok {
		return op.Rollback(ctx, fsys)
	}
	return nil
}

func (soa *simpleOperationAdapter) GetItem() interface{} {
	if op, ok := soa.op.(interface{ GetItem() interface{} }); ok {
		return op.GetItem()
	}
	return nil
}

func (soa *simpleOperationAdapter) SetDescriptionDetail(key string, value interface{}) {
	if op, ok := soa.op.(interface{ SetDescriptionDetail(string, interface{}) }); ok {
		op.SetDescriptionDetail(key, value)
	}
}

// noOpLogger is a simple no-op implementation of Logger for when none is provided
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