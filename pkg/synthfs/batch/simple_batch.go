package batch

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// SimpleBatchImpl is a simplified batch implementation that doesn't handle prerequisites
// It creates operations without hardcoded parent directory logic
type SimpleBatchImpl struct {
	operations []interface{}
	fs         interface{} // Filesystem interface
	ctx        context.Context
	idCounter  int
	registry   core.OperationFactory
	logger     core.Logger
}

// NewSimpleBatch creates a new simplified operation batch
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

// Operations returns all operations currently in the batch
func (sb *SimpleBatchImpl) Operations() []interface{} {
	opsCopy := make([]interface{}, len(sb.operations))
	copy(opsCopy, sb.operations)
	return opsCopy
}

// WithFileSystem sets the filesystem for the batch operations
func (sb *SimpleBatchImpl) WithFileSystem(fs interface{}) Batch {
	sb.fs = fs
	return sb
}

// WithContext sets the context for the batch operations
func (sb *SimpleBatchImpl) WithContext(ctx context.Context) Batch {
	sb.ctx = ctx
	return sb
}

// WithRegistry sets a custom operation registry for the batch
func (sb *SimpleBatchImpl) WithRegistry(registry core.OperationFactory) Batch {
	sb.registry = registry
	return sb
}

// WithLogger sets the logger for the batch
func (sb *SimpleBatchImpl) WithLogger(logger core.Logger) Batch {
	sb.logger = logger
	return sb
}

// CreateDir adds a directory creation operation to the batch
func (sb *SimpleBatchImpl) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0755)
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	op, err := sb.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	dirItem := targets.NewDirectory(path).WithMode(fileMode)
	if err := sb.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateDir operation: %w", err)
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"mode": fileMode.String(),
	}); err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// CreateFile adds a file creation operation to the batch
func (sb *SimpleBatchImpl) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0644)
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	op, err := sb.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	fileItem := targets.NewFile(path).WithContent(content).WithMode(fileMode)
	if err := sb.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateFile operation: %w", err)
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"content_length": len(content),
		"mode":           fileMode.String(),
		"content":        content,
	}); err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// Copy adds a copy operation to the batch
func (sb *SimpleBatchImpl) Copy(src, dst string) (interface{}, error) {
	op, err := sb.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	if err := sb.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Compute checksum for source file if possible
	if fs, ok := sb.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			if setter, ok := op.(interface{ SetChecksum(string, interface{}) }); ok {
				setter.SetChecksum(src, checksum)
			}
			_ = sb.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// Move adds a move operation to the batch
func (sb *SimpleBatchImpl) Move(src, dst string) (interface{}, error) {
	op, err := sb.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	if err := sb.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Compute checksum for source file if possible
	if fs, ok := sb.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			if setter, ok := op.(interface{ SetChecksum(string, interface{}) }); ok {
				setter.SetChecksum(src, checksum)
			}
			_ = sb.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// Delete adds a delete operation to the batch
func (sb *SimpleBatchImpl) Delete(path string) (interface{}, error) {
	op, err := sb.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch
func (sb *SimpleBatchImpl) CreateSymlink(target, linkPath string) (interface{}, error) {
	op, err := sb.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	symlinkItem := targets.NewSymlink(linkPath, target)
	if err := sb.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateSymlink operation: %w", err)
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"target": target,
	}); err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// CreateArchive adds an archive creation operation to the batch
func (sb *SimpleBatchImpl) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
	op, err := sb.createOperation("create_archive", archivePath)
	if err != nil {
		return nil, err
	}

	archiveItem := targets.NewArchive(archivePath, format, sources...)
	if err := sb.registry.SetItemForOperation(op, archiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateArchive operation: %w", err)
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"format":       format,
		"sources":      sources,
		"source_count": len(sources),
	}); err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// Unarchive adds an unarchive operation to the batch
func (sb *SimpleBatchImpl) Unarchive(archivePath, extractPath string) (interface{}, error) {
	op, err := sb.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	unarchiveItem := targets.NewUnarchive(archivePath, extractPath)
	if err := sb.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for Unarchive operation: %w", err)
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"extract_path": extractPath,
	}); err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch
func (sb *SimpleBatchImpl) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
	op, err := sb.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	unarchiveItem := targets.NewUnarchive(archivePath, extractPath).WithPatterns(patterns...)
	if err := sb.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for UnarchiveWithPatterns operation: %w", err)
	}

	if err := sb.setOperationDetails(op, map[string]interface{}{
		"extract_path":  extractPath,
		"patterns":      patterns,
		"pattern_count": len(patterns),
	}); err != nil {
		return nil, err
	}

	sb.operations = append(sb.operations, op)
	return op, nil
}

// Run runs all operations with prerequisite resolution enabled by default
func (sb *SimpleBatchImpl) Run() (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            false,
		"max_backup_size_mb":    0,
	}
	return sb.RunWithOptions(opts)
}

// RunWithOptions runs all operations with specified options
func (sb *SimpleBatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	// Convert opts to map if needed
	var optsMap map[string]interface{}
	if o, ok := opts.(map[string]interface{}); ok {
		optsMap = o
	} else {
		optsMap = map[string]interface{}{}
	}

	// Create execution pipeline
	pipeline := execution.NewPipeline()
	
	// Add operations to pipeline
	for _, op := range sb.operations {
		adapter := &simpleOperationAdapter{op: op}
		if err := pipeline.Add(adapter); err != nil {
			return nil, fmt.Errorf("failed to add operation to pipeline: %w", err)
		}
	}

	// Create executor
	executor := execution.NewExecutor(sb.logger)

	// Convert options to PipelineOptions
	pipelineOpts := core.PipelineOptions{
		Restorable:           false,
		MaxBackupSizeMB:      0,
		ResolvePrerequisites: true,
	}

	if restorable, ok := optsMap["restorable"].(bool); ok {
		pipelineOpts.Restorable = restorable
	}

	if maxBackupMB, ok := optsMap["max_backup_size_mb"].(int); ok {
		pipelineOpts.MaxBackupSizeMB = maxBackupMB
	}

	if resolvePrereqs, ok := optsMap["resolve_prerequisites"].(bool); ok {
		pipelineOpts.ResolvePrerequisites = resolvePrereqs
	}

	// Create prerequisite resolver if needed
	var prereqResolver core.PrerequisiteResolver
	if pipelineOpts.ResolvePrerequisites {
		prereqResolver = execution.NewPrerequisiteResolver(sb.registry, sb.logger)
	}

	// Execute using the execution package
	coreResult := executor.RunWithOptionsAndResolver(sb.ctx, pipeline, sb.fs, pipelineOpts, prereqResolver)

	// Convert core.Result back to our interface{} result
	var executionError error
	if !coreResult.Success && len(coreResult.Errors) > 0 {
		executionError = coreResult.Errors[0]
	}

	var restoreOps []interface{}
	if coreResult.RestoreOps != nil {
		restoreOps = coreResult.RestoreOps
	}

	var operationResults []interface{}
	for _, opResult := range coreResult.Operations {
		operationResults = append(operationResults, opResult)
	}

	batchResult := NewResultWithBudgetAndRollback(
		coreResult.Success,
		operationResults,
		restoreOps,
		coreResult.Duration,
		executionError,
		coreResult.Budget,
		coreResult.Rollback,
	)

	return batchResult, nil
}

// RunRestorable runs all operations with backup enabled using the default 10MB budget
func (sb *SimpleBatchImpl) RunRestorable() (interface{}, error) {
	return sb.RunRestorableWithBudget(10)
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget
func (sb *SimpleBatchImpl) RunRestorableWithBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
		"resolve_prerequisites": true,
	}
	return sb.RunWithOptions(opts)
}

// RunWithPrerequisites runs all operations with prerequisite resolution enabled
func (sb *SimpleBatchImpl) RunWithPrerequisites() (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            false,
		"max_backup_size_mb":    0,
	}
	return sb.RunWithOptions(opts)
}

// RunWithPrerequisitesAndBudget runs all operations with prerequisite resolution and backup enabled
func (sb *SimpleBatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
	}
	return sb.RunWithOptions(opts)
}

// Helper methods

// generateID creates a unique operation ID based on type and path
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

// simpleOperationAdapter wraps interface{} operations to implement execution.OperationInterface
type simpleOperationAdapter struct {
	op interface{}
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
	if op, ok := soa.op.(interface{ Validate(context.Context, interface{}) error }); ok {
		if ctxTyped, ok := ctx.(context.Context); ok {
			return op.Validate(ctxTyped, fsys)
		}
	}
	return nil
}

func (soa *simpleOperationAdapter) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
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