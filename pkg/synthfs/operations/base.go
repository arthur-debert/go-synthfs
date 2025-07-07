package operations

import (
	"context"
	"fmt"
	"time"
	
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// BaseOperation provides a base implementation of the Operation interface.
// Operations are created complete and immutable - no post-creation modification.
type BaseOperation struct {
	id           core.OperationID
	dependencies []core.OperationID
	description  core.OperationDesc
	item         interface{}                     // Generic item interface
	srcPath      string                         // For Copy/Move operations
	dstPath      string                         // For Copy/Move operations
	checksums    map[string]interface{}         // Generic checksum storage
}

// NewBaseOperation creates a new base operation.
func NewBaseOperation(id core.OperationID, descType string, path string) *BaseOperation {
	return &BaseOperation{
		id: id,
		description: core.OperationDesc{
			Type:    descType,
			Path:    path,
			Details: make(map[string]interface{}),
		},
		dependencies: []core.OperationID{},
		checksums:    make(map[string]interface{}),
	}
}

// ID returns the operation's ID.
func (op *BaseOperation) ID() core.OperationID {
	return op.id
}

// Dependencies returns the list of operation dependencies.
func (op *BaseOperation) Dependencies() []core.OperationID {
	return op.dependencies
}

// Conflicts returns an empty list (conflicts not implemented yet).
func (op *BaseOperation) Conflicts() []core.OperationID {
	return nil
}

// Describe returns the operation's description.
func (op *BaseOperation) Describe() core.OperationDesc {
	return op.description
}

// GetItem returns the item associated with this operation.
func (op *BaseOperation) GetItem() interface{} {
	return op.item
}

// SetItem sets the item for operations.
func (op *BaseOperation) SetItem(item interface{}) {
	op.item = item
}

// GetPaths returns source and destination paths.
func (op *BaseOperation) GetPaths() (src, dst string) {
	return op.srcPath, op.dstPath
}

// SetPaths sets source and destination paths for Copy/Move operations.
func (op *BaseOperation) SetPaths(src, dst string) {
	op.srcPath = src
	op.dstPath = dst
}

// AddDependency adds a dependency to the operation.
func (op *BaseOperation) AddDependency(depID core.OperationID) {
	op.dependencies = append(op.dependencies, depID)
}

// SetDescriptionDetail sets a detail in the operation's description.
func (op *BaseOperation) SetDescriptionDetail(key string, value interface{}) {
	if op.description.Details == nil {
		op.description.Details = make(map[string]interface{})
	}
	op.description.Details[key] = value
}

// SetChecksum stores a checksum record for a file path
func (op *BaseOperation) SetChecksum(path string, checksum interface{}) {
	if op.checksums == nil {
		op.checksums = make(map[string]interface{})
	}
	op.checksums[path] = checksum
}

// GetChecksum retrieves a checksum record for a file path
func (op *BaseOperation) GetChecksum(path string) interface{} {
	if op.checksums == nil {
		return nil
	}
	return op.checksums[path]
}

// GetAllChecksums returns all checksum records
func (op *BaseOperation) GetAllChecksums() map[string]interface{} {
	return op.checksums
}

// Execute performs the actual filesystem operation.
// Subclasses should override this method.
func (op *BaseOperation) Execute(ctx context.Context, fsys interface{}) error {
	return fmt.Errorf("Execute not implemented for operation type: %s", op.description.Type)
}

// Validate checks if the operation can be performed.
// Subclasses should override this method.
func (op *BaseOperation) Validate(ctx context.Context, fsys interface{}) error {
	// Basic validation: reject empty paths
	if op.description.Path == "" {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "path cannot be empty",
			Cause:         nil,
		}
	}
	return nil
}

// Rollback attempts to undo the effects of the Execute method.
// Subclasses should override this method.
func (op *BaseOperation) Rollback(ctx context.Context, fsys interface{}) error {
	return fmt.Errorf("Rollback not implemented for operation type: %s", op.description.Type)
}

// ReverseOps generates operations that would undo this operation's effects.
// Subclasses should override this method.
func (op *BaseOperation) ReverseOps(ctx context.Context, fsys interface{}, budget interface{}) ([]interface{}, interface{}, error) {
	return nil, nil, fmt.Errorf("ReverseOps not implemented for operation type: %s", op.description.Type)
}

// ExecuteV2 performs the operation using ExecutionContext.
func (op *BaseOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert interfaces back to concrete types
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
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
	
	err := op.Execute(context, fsys)
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

// ValidateV2 checks if the operation can be performed using ExecutionContext.
func (op *BaseOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert interfaces back to concrete types
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}
	
	// For now, delegate to the original method
	return op.Validate(context, fsys)
}