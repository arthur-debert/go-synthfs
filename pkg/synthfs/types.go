package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// --- Core Interface Types ---

// Filesystem type aliases for backward compatibility
// The actual interfaces are defined in the filesystem package
type ReadFS = filesystem.ReadFS
type WriteFS = filesystem.WriteFS
type FileSystem = filesystem.FileSystem
type StatFS = filesystem.StatFS
type FullFileSystem = filesystem.FullFileSystem

// --- FsItem Types ---

// FsItem represents a filesystem item to be created.
type FsItem interface {
	Path() string
	Type() string
}

// --- Operation Types ---

// Type aliases for core types
type OperationID = core.OperationID
type OperationDesc = core.OperationDesc
type BackupData = core.BackupData

// ChecksumRecord is now defined in the validation package
type ChecksumRecord = validation.ChecksumRecord

type BackedUpItem struct {
	RelativePath string
	ItemType     string
	Mode         fs.FileMode
	Content      []byte
	Size         int64
	ModTime      time.Time
}

// BackupBudget is now defined in the core package
type BackupBudget = core.BackupBudget

// Executable defines execution capabilities for operations
type Executable interface {
	Execute(ctx context.Context, fsys FileSystem) error
	Validate(ctx context.Context, fsys FileSystem) error
}

// Operation is the main interface that composes all operation capabilities
type Operation interface {
	core.OperationMetadata  // ID(), Describe()
	core.DependencyAware    // Dependencies(), Conflicts()
	Executable              // Execute(), Validate()
	core.ExecutableV2       // ExecuteV2(), ValidateV2() - new methods
	Rollback(ctx context.Context, fsys FileSystem) error
	GetItem() FsItem
	GetChecksum(path string) *ChecksumRecord
	GetAllChecksums() map[string]*ChecksumRecord
	ReverseOps(ctx context.Context, fsys FileSystem, budget *core.BackupBudget) ([]Operation, *core.BackupData, error)
	
	// Batch building methods
	SetDescriptionDetail(key string, value interface{})
	AddDependency(depID OperationID)
	SetPaths(src, dst string)
}

// ValidationError represents an error during operation validation.
type ValidationError struct {
	Operation Operation
	Reason    string
	Cause     error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error for operation %s (%s): %s: %v",
			e.Operation.ID(), e.Operation.Describe().Path, e.Reason, e.Cause)
	}
	return fmt.Sprintf("validation error for operation %s (%s): %s",
		e.Operation.ID(), e.Operation.Describe().Path, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}
