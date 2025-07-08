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

// SimpleBatchImpl represents a simplified batch implementation without parent directory auto-creation.
type SimpleBatchImpl struct {
	operations []interface{}
	fs         interface{}
	ctx        context.Context
	idCounter  int
	registry   core.OperationFactory
	logger     core.Logger
}

// NewSimpleBatch creates a new simplified operation batch.
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
	// Only basic validation - no parent directory auto-creation
	if err := b.validateOperation(op); err != nil {
		return err
	}
	b.operations = append(b.operations, op)
	return nil
}

// validateOperation performs basic validation only
func (b *SimpleBatchImpl) validateOperation(op interface{}) error {
	if v, ok := op.(interface{ Validate(context.Context, interface{}) error }); ok {
		return v.Validate(b.ctx, b.fs)
	}
	if v, ok := op.(interface{ ValidateV2(interface{}, *core.ExecutionContext, interface{}) error }); ok {
		execCtx := &core.ExecutionContext{}
		return v.ValidateV2(b.ctx, execCtx, b.fs)
	}
	return nil
}

// CreateDir adds a directory creation operation to the batch.
func (b *SimpleBatchImpl) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0755)
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	op, err := b.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	dirItem := targets.NewDirectory(path).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateDir operation: %w", err)
	}

	if err := b.setOperationDetails(op, map[string]interface{}{"mode": fileMode.String()}); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
func (b *SimpleBatchImpl) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0644)
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	op, err := b.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	fileItem := targets.NewFile(path).WithContent(content).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateFile operation: %w", err)
	}

	if err := b.setOperationDetails(op, map[string]interface{}{
		"content_length": len(content),
		"mode":           fileMode.String(),
		"content":        content,
	}); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	return op, nil
}

// Copy adds a copy operation to the batch.
func (b *SimpleBatchImpl) Copy(src, dst string) (interface{}, error) {
	op, err := b.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	// Compute checksum if possible
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			if setter, ok := op.(interface{ SetChecksum(string, *validation.ChecksumRecord) }); ok {
				setter.SetChecksum(src, checksum)
			}
		}
	}

	return op, nil
}

// Move adds a move operation to the batch.
func (b *SimpleBatchImpl) Move(src, dst string) (interface{}, error) {
	op, err := b.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	return op, nil
}

// Delete adds a delete operation to the batch.
func (b *SimpleBatchImpl) Delete(path string) (interface{}, error) {
	op, err := b.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete(%s): %w", path, err)
	}

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (b *SimpleBatchImpl) CreateSymlink(target, linkPath string) (interface{}, error) {
	op, err := b.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	symlinkItem := targets.NewSymlink(linkPath, target)
	if err := b.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateSymlink operation: %w", err)
	}

	if err := b.setOperationDetails(op, map[string]interface{}{"target": target}); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
func (b *SimpleBatchImpl) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("validation failed for CreateArchive(%s): must specify at least one source", archivePath)
	}

	op, err := b.createOperation("create_archive", archivePath)
	if err != nil {
		return nil, err
	}

	if err := b.setOperationDetails(op, map[string]interface{}{
		"format":       format,
		"source_count": len(sources),
		"sources":      sources,
	}); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateArchive(%s): %w", archivePath, err)
	}

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
func (b *SimpleBatchImpl) Unarchive(archivePath, extractPath string) (interface{}, error) {
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	unarchiveItem := targets.NewUnarchive(archivePath, extractPath)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for Unarchive operation: %w", err)
	}

	if err := b.setOperationDetails(op, map[string]interface{}{"extract_path": extractPath}); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
func (b *SimpleBatchImpl) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	unarchiveItem := targets.NewUnarchive(archivePath, extractPath).WithPatterns(patterns...)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for UnarchiveWithPatterns operation: %w", err)
	}

	if err := b.setOperationDetails(op, map[string]interface{}{
		"extract_path":  extractPath,
		"patterns":      patterns,
		"pattern_count": len(patterns),
	}); err != nil {
		return nil, err
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add UnarchiveWithPatterns(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// Run runs all operations in the batch with SimpleBatch defaults (prerequisite resolution enabled).
func (b *SimpleBatchImpl) Run() (interface{}, error) {
	defaultOpts := map[string]interface{}{
		"restorable":            false,
		"max_backup_size_mb":    0,
		"resolve_prerequisites": true, // SimpleBatch enables prerequisites by default
	}
	return b.RunWithOptions(defaultOpts)
}

// RunWithOptions runs all operations in the batch with specified options.
func (b *SimpleBatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	startTime := time.Now()

	pipelineOpts := core.PipelineOptions{
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: true, // SimpleBatch default
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

	if b.logger != nil {
		b.logger.Info().
			Int("operation_count", len(b.operations)).
			Bool("restorable", pipelineOpts.Restorable).
			Bool("resolve_prerequisites", pipelineOpts.ResolvePrerequisites).
			Msg("executing simple batch")
	}

	if len(b.operations) == 0 {
		duration := time.Since(startTime)
		return NewResult(true, b.operations, []interface{}{}, duration, nil), nil
	}

	loggerToUse := b.logger
	if loggerToUse == nil {
		loggerToUse = &noOpLogger{}
	}

	executor := execution.NewExecutor(loggerToUse)
	pipeline := execution.NewMemPipeline(loggerToUse)

	// Add operations to pipeline with adapters
	for _, op := range b.operations {
		adapter := &simpleOperationAdapter{op: op}
		if err := pipeline.Add(adapter); err != nil {
			return NewResult(false, []interface{}{}, []interface{}{}, time.Since(startTime), err), 
				fmt.Errorf("failed to add operation to pipeline: %w", err)
		}
	}

	var prereqResolver core.PrerequisiteResolver
	if pipelineOpts.ResolvePrerequisites {
		prereqResolver = execution.NewPrerequisiteResolver(b.registry, loggerToUse)
	}

	coreResult := executor.RunWithOptionsAndResolver(b.ctx, pipeline, b.fs, pipelineOpts, prereqResolver)
	duration := time.Since(startTime)

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

	return NewResultWithBudgetAndRollback(
		coreResult.Success,
		operationResults,
		restoreOps,
		duration,
		executionError,
		coreResult.Budget,
		coreResult.Rollback,
	), nil
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

// generateID creates a unique operation ID based on type and path.
func (b *SimpleBatchImpl) generateID(opType, path string) core.OperationID {
	b.idCounter++
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	return core.OperationID(fmt.Sprintf("simple_batch_%d_%s_%s", b.idCounter, opType, cleanPath))
}

// createOperation is a helper method to create operations using the registry
func (b *SimpleBatchImpl) createOperation(opType, path string) (interface{}, error) {
	opID := b.generateID(opType, path)
	return b.registry.CreateOperation(opID, opType, path)
}

// setOperationDetails sets details on an operation through interface assertion
func (b *SimpleBatchImpl) setOperationDetails(op interface{}, details map[string]interface{}) error {
	if setter, ok := op.(interface{ SetDescriptionDetail(string, interface{}) }); ok {
		for key, value := range details {
			setter.SetDescriptionDetail(key, value)
		}
	}
	return nil
}

// setOperationPaths sets paths on an operation through interface assertion
func (b *SimpleBatchImpl) setOperationPaths(op interface{}, src, dst string) error {
	if setter, ok := op.(interface{ SetPaths(string, string) }); ok {
		setter.SetPaths(src, dst)
	}
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
	if op, ok := soa.op.(interface{ ReverseOps(context.Context, interface{}, interface{}) ([]interface{}, interface{}, error) }); ok {
		ops, backupData, err := op.ReverseOps(ctx, fsys, budget)
		var bd *core.BackupData
		if backupData != nil {
			if typed, ok := backupData.(*core.BackupData); ok {
				bd = typed
			}
		}
		return ops, bd, err
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

// Prerequisites returns the prerequisites for this operation
func (soa *simpleOperationAdapter) Prerequisites() []core.Prerequisite {
	if op, ok := soa.op.(interface{ Prerequisites() []core.Prerequisite }); ok {
		return op.Prerequisites()
	}
	return nil
}