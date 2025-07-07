package synthfs

import (
	"context"
	"fmt"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// SimpleOperation provides a straightforward implementation of Operation.
// Operations are created complete and immutable - no post-creation modification.
type SimpleOperation struct {
	id           OperationID
	dependencies []OperationID
	description  OperationDesc
	item         FsItem                     // For Create operations
	srcPath      string                     // For Copy/Move operations
	dstPath      string                     // For Copy/Move operations
	checksums    map[string]*ChecksumRecord // Phase I, Milestone 3: Store file checksums
}

// NewSimpleOperation creates a new simple operation.
func NewSimpleOperation(id OperationID, descType string, path string) *SimpleOperation {
	return &SimpleOperation{
		id: id,
		description: OperationDesc{
			Type:    descType,
			Path:    path,
			Details: make(map[string]interface{}),
		},
		dependencies: []OperationID{},
		checksums:    make(map[string]*ChecksumRecord), // Phase I, Milestone 3: Initialize checksum storage
	}
}

// ID returns the operation's ID.
func (op *SimpleOperation) ID() OperationID {
	return op.id
}

// Dependencies returns the list of operation dependencies.
func (op *SimpleOperation) Dependencies() []OperationID {
	return op.dependencies
}

// Conflicts returns an empty list (conflicts not implemented yet).
func (op *SimpleOperation) Conflicts() []OperationID {
	return nil
}

// Describe returns the operation's description.
func (op *SimpleOperation) Describe() OperationDesc {
	return op.description
}

// GetItem returns the FsItem associated with this operation.
func (op *SimpleOperation) GetItem() FsItem {
	return op.item
}

// SetItem sets the FsItem for Create operations.
func (op *SimpleOperation) SetItem(item FsItem) {
	op.item = item
}

// SetPaths sets source and destination paths for Copy/Move operations.
func (op *SimpleOperation) SetPaths(src, dst string) {
	op.srcPath = src
	op.dstPath = dst
}

// AddDependency adds a dependency to the operation.
func (op *SimpleOperation) AddDependency(depID OperationID) {
	op.dependencies = append(op.dependencies, depID)
}

// SetDescriptionDetail sets a detail in the operation's description.
func (op *SimpleOperation) SetDescriptionDetail(key string, value interface{}) {
	if op.description.Details == nil {
		op.description.Details = make(map[string]interface{})
	}
	op.description.Details[key] = value
}

// GetSrcPath returns the source path for copy/move operations.
func (op *SimpleOperation) GetSrcPath() string {
	return op.srcPath
}

// GetDstPath returns the destination path for copy/move operations.
func (op *SimpleOperation) GetDstPath() string {
	return op.dstPath
}

// SetChecksum stores a checksum record for a file path (Phase I, Milestone 3)
func (op *SimpleOperation) SetChecksum(path string, checksum *ChecksumRecord) {
	if op.checksums == nil {
		op.checksums = make(map[string]*ChecksumRecord)
	}
	op.checksums[path] = checksum
}

// GetChecksum retrieves a checksum record for a file path (Phase I, Milestone 3)
func (op *SimpleOperation) GetChecksum(path string) *ChecksumRecord {
	if op.checksums == nil {
		return nil
	}
	return op.checksums[path]
}

// GetAllChecksums returns all checksum records (Phase I, Milestone 3)
func (op *SimpleOperation) GetAllChecksums() map[string]*ChecksumRecord {
	return op.checksums
}

// verifyChecksums verifies all stored checksums against current file state (Phase I, Milestone 4)
func (op *SimpleOperation) verifyChecksums(ctx context.Context, fsys FileSystem) error {
	if len(op.checksums) == 0 {
		return nil // No checksums to verify
	}

	// Check if filesystem supports Stat operation
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		// If filesystem doesn't support Stat, we cannot compute a checksum.
		// Log a warning and skip verification.
		Logger().Warn().
			Str("op_id", string(op.ID())).
			Msg("skipping checksum verification: filesystem does not support Stat")
		return nil
	}

	for path, expectedChecksum := range op.checksums {
		// Re-compute the checksum for the current file state
		currentChecksum, err := ComputeFileChecksum(fullFS, path)
		if err != nil {
			return fmt.Errorf("checksum verification failed for %s: could not compute current checksum: %w", path, err)
		}

		// It's possible for a file to be replaced by a directory
		if currentChecksum == nil && expectedChecksum != nil {
			return fmt.Errorf("checksum verification failed for %s: expected a file but found a directory", path)
		}

		// Compare the MD5 hashes
		if currentChecksum.MD5 != expectedChecksum.MD5 {
			return fmt.Errorf("checksum verification failed for %s: file content has changed. Expected MD5: %s, got: %s",
				path, expectedChecksum.MD5, currentChecksum.MD5)
		}

		// Optional: We could still log if modtime/size differ but hash is same, but for now hash equality is sufficient.
		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("path", path).
			Str("md5", currentChecksum.MD5).
			Msg("checksum verification passed")
	}

	return nil
}

// ExecuteV2 performs the actual filesystem operation using ExecutionContext.
func (op *SimpleOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert interfaces back to concrete types
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}
	
	filesystem, ok := fsys.(FileSystem)
	if !ok {
		return fmt.Errorf("invalid filesystem type")
	}
	
	// Emit operation started event
	if execCtx.EventBus != nil {
		startEvent := core.NewOperationStartedEvent(
			op.id,
			op.description.Type,
			op.description.Path,
			op.description.Details,
		)
		execCtx.EventBus.PublishAsync(context, startEvent)
	}
	
	// Execute the operation and measure duration
	startTime := time.Now()
	
	execCtx.Logger.Trace().
		Str("op_id", string(op.id)).
		Str("op_type", op.description.Type).
		Str("path", op.description.Path).
		Msg("executing operation")
	
	err := op.Execute(context, filesystem)
	duration := time.Since(startTime)
	
	// Emit completion or failure event
	if execCtx.EventBus != nil {
		if err != nil {
			failEvent := core.NewOperationFailedEvent(
				op.id,
				op.description.Type,
				op.description.Path,
				op.description.Details,
				err,
				duration,
			)
			execCtx.EventBus.PublishAsync(context, failEvent)
		} else {
			completeEvent := core.NewOperationCompletedEvent(
				op.id,
				op.description.Type,
				op.description.Path,
				op.description.Details,
				duration,
			)
			execCtx.EventBus.PublishAsync(context, completeEvent)
		}
	}
	
	if err != nil {
		execCtx.Logger.Trace().
			Str("op_id", string(op.id)).
			Str("op_type", op.description.Type).
			Str("path", op.description.Path).
			Dur("duration", duration).
			Err(err).
			Msg("operation failed")
	} else {
		execCtx.Logger.Trace().
			Str("op_id", string(op.id)).
			Str("op_type", op.description.Type).
			Str("path", op.description.Path).
			Dur("duration", duration).
			Msg("operation completed")
	}
	
	return err
}

// Execute performs the actual filesystem operation.
func (op *SimpleOperation) Execute(ctx context.Context, fsys FileSystem) error {
	switch op.description.Type {
	case "create_file":
		return op.executeCreateFile(ctx, fsys)
	case "create_directory":
		return op.executeCreateDirectory(ctx, fsys)
	case "create_symlink":
		return op.executeCreateSymlink(ctx, fsys)
	case "create_archive":
		return op.executeCreateArchive(ctx, fsys)
	case "unarchive":
		return op.executeUnarchive(ctx, fsys)
	case "copy":
		return op.executeCopy(ctx, fsys)
	case "move":
		return op.executeMove(ctx, fsys)
	case "delete":
		return op.executeDelete(ctx, fsys)
	default:
		return fmt.Errorf("unknown operation type: %s", op.description.Type)
	}
}

// ValidateV2 checks if the operation can be performed using ExecutionContext.
func (op *SimpleOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert interfaces back to concrete types
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}
	
	filesystem, ok := fsys.(FileSystem)
	if !ok {
		return fmt.Errorf("invalid filesystem type")
	}
	
	// For now, delegate to the original method
	// In the future, we'll use execCtx.Logger instead of global Logger()
	return op.Validate(context, filesystem)
}

// Validate checks if the operation can be performed.
func (op *SimpleOperation) Validate(ctx context.Context, fsys FileSystem) error {
	// Basic validation: reject empty paths
	if op.description.Path == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "path cannot be empty",
			Cause:     nil,
		}
	}

	switch op.description.Type {
	case "create_file":
		return op.validateCreateFile(ctx, fsys)
	case "create_directory":
		return op.validateCreateDirectory(ctx, fsys)
	case "create_symlink":
		return op.validateCreateSymlink(ctx, fsys)
	case "create_archive":
		return op.validateCreateArchive(ctx, fsys)
	case "unarchive":
		return op.validateUnarchive(ctx, fsys)
	case "copy":
		return op.validateCopy(ctx, fsys)
	case "move":
		return op.validateMove(ctx, fsys)
	case "delete":
		return op.validateDelete(ctx, fsys)
	default:
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("unknown operation type: %s", op.description.Type),
		}
	}
}

// Rollback attempts to undo the effects of the Execute method.
func (op *SimpleOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	switch op.description.Type {
	case "create_file", "create_directory", "create_symlink", "create_archive":
		// For create operations, rollback means removing what was created
		return op.rollbackCreate(ctx, fsys)
	case "unarchive":
		// For unarchive operations, rollback means removing extracted files
		return op.rollbackUnarchive(ctx, fsys)
	case "copy":
		// For copy operations, rollback means removing the destination
		return op.rollbackCopy(ctx, fsys)
	case "move":
		// For move operations, rollback means moving back
		return op.rollbackMove(ctx, fsys)
	case "delete":
		// For delete operations, rollback is complex - would need to restore
		// For now, we'll return an error indicating rollback isn't supported
		return fmt.Errorf("rollback of delete operations not yet implemented")
	default:
		return fmt.Errorf("unknown operation type for rollback: %s", op.description.Type)
	}
}

// ReverseOps generates operations that would undo this operation's effects (Phase III)
func (op *SimpleOperation) ReverseOps(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	switch op.description.Type {
	case "create_file":
		return op.reverseCreateFile(ctx, fsys, budget)
	case "create_directory":
		return op.reverseCreateDirectory(ctx, fsys, budget)
	case "create_symlink":
		return op.reverseCreateSymlink(ctx, fsys, budget)
	case "create_archive":
		return op.reverseCreateArchive(ctx, fsys, budget)
	case "unarchive":
		return op.reverseUnarchive(ctx, fsys, budget)
	case "copy":
		return op.reverseCopy(ctx, fsys, budget)
	case "move":
		return op.reverseMove(ctx, fsys, budget)
	case "delete":
		return op.reverseDelete(ctx, fsys, budget)
	default:
		return nil, nil, fmt.Errorf("unknown operation type for reverse ops: %s", op.description.Type)
	}
}