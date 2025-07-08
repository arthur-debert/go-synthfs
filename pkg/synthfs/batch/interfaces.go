package batch

import (
	"context"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// BatchOptions controls how batch operations are executed
type BatchOptions struct {
	UseSimpleBatch bool // When true, use SimpleBatch + prerequisite resolution; when false (default), use existing behavior
}

// Batch represents a collection of operations that can be validated and executed as a unit.
type Batch interface {
	// Operation management
	Operations() []interface{}

	// Operation creation methods
	CreateDir(path string, mode ...fs.FileMode) (interface{}, error)
	CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error)
	Copy(src, dst string) (interface{}, error)
	Move(src, dst string) (interface{}, error)
	Delete(path string) (interface{}, error)
	CreateSymlink(target, linkPath string) (interface{}, error)
	CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error)
	Unarchive(archivePath, extractPath string) (interface{}, error)
	UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error)

	// Configuration
	WithFileSystem(fs interface{}) Batch
	WithContext(ctx context.Context) Batch
	WithRegistry(registry core.OperationFactory) Batch
	WithLogger(logger core.Logger) Batch
	WithOptions(opts BatchOptions) Batch

	// Execution
	Run() (interface{}, error)
	RunWithOptions(opts interface{}) (interface{}, error)
	RunRestorable() (interface{}, error)
	RunRestorableWithBudget(maxBackupMB int) (interface{}, error)
}

// Result represents the outcome of executing a batch of operations
type Result interface {
	IsSuccess() bool
	GetOperations() []interface{}
	GetRestoreOps() []interface{}
	GetDuration() interface{}
	GetError() error
	GetBudget() interface{} // Budget information from execution (may be nil for non-restorable runs)
	GetRollback() interface{} // Rollback function (func(context.Context) error)
}
