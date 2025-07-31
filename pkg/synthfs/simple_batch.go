package synthfs

import (
	"context"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// SimpleBatch provides a fluent API for building and executing multiple operations
// This is a convenience wrapper that doesn't require a registry
type SimpleBatch struct {
	fs         filesystem.FileSystem
	operations []Operation
	ctx        context.Context
}

// NewSimpleBatch creates a new simple batch for the given filesystem
func NewSimpleBatch(fs filesystem.FileSystem) *SimpleBatch {
	return &SimpleBatch{
		fs:         fs,
		operations: []Operation{},
		ctx:        context.Background(),
	}
}

// WithContext sets the context for batch execution
func (sb *SimpleBatch) WithContext(ctx context.Context) *SimpleBatch {
	sb.ctx = ctx
	return sb
}

// CreateDir adds a directory creation operation to the batch
func (sb *SimpleBatch) CreateDir(path string, mode fs.FileMode) *SimpleBatch {
	op := CreateDir(path, mode)
	sb.operations = append(sb.operations, op)
	return sb
}

// WriteFile adds a file write operation to the batch
func (sb *SimpleBatch) WriteFile(path string, content []byte, mode fs.FileMode) *SimpleBatch {
	op := CreateFile(path, content, mode)
	sb.operations = append(sb.operations, op)
	return sb
}

// Copy adds a copy operation to the batch
func (sb *SimpleBatch) Copy(src, dst string) *SimpleBatch {
	op := Copy(src, dst)
	sb.operations = append(sb.operations, op)
	return sb
}

// Move adds a move operation to the batch
func (sb *SimpleBatch) Move(src, dst string) *SimpleBatch {
	op := Move(src, dst)
	sb.operations = append(sb.operations, op)
	return sb
}

// Delete adds a delete operation to the batch
func (sb *SimpleBatch) Delete(path string) *SimpleBatch {
	op := Delete(path)
	sb.operations = append(sb.operations, op)
	return sb
}

// CreateSymlink adds a symlink creation operation to the batch
func (sb *SimpleBatch) CreateSymlink(target, linkPath string) *SimpleBatch {
	op := CreateSymlink(target, linkPath)
	sb.operations = append(sb.operations, op)
	return sb
}

// Execute runs all operations in the batch
func (sb *SimpleBatch) Execute() error {
	_, err := Run(sb.ctx, sb.fs, sb.operations...)
	return err
}

// ExecuteWithRollback runs all operations and attempts rollback on failure
func (sb *SimpleBatch) ExecuteWithRollback() error {
	err := sb.Execute()
	if err == nil {
		return nil
	}
	
	// Attempt rollback
	rollbackErrs := make(map[OperationID]error)
	
	// Roll back in reverse order
	for i := len(sb.operations) - 1; i >= 0; i-- {
		op := sb.operations[i]
		if rollbackErr := op.Rollback(sb.ctx, sb.fs); rollbackErr != nil {
			// Ignore "not found" errors during rollback as they're expected
			rollbackErrs[op.ID()] = rollbackErr
		}
	}
	
	if len(rollbackErrs) > 0 {
		return &RollbackError{
			OriginalErr:  err,
			RollbackErrs: rollbackErrs,
		}
	}
	
	return err
}

// Operations returns the list of operations in the batch
func (sb *SimpleBatch) Operations() []Operation {
	return sb.operations
}

// Clear removes all operations from the batch
func (sb *SimpleBatch) Clear() *SimpleBatch {
	sb.operations = []Operation{}
	return sb
}