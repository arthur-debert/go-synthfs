package synthfs

import (
	"context"
	"io/fs"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// --- Core Interface Types ---

// Filesystem type aliases for backward compatibility
// The actual interfaces are defined in the filesystem package
type ReadFS = filesystem.ReadFS
type WriteFS = filesystem.WriteFS
type FileSystem = filesystem.FileSystem
// Phase 2: Legacy aliases StatFS and FullFileSystem have been removed - use FileSystem directly

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
type Result = core.Result
type OperationResult = core.OperationResult

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

// Operation type alias - Phase 2: Use operations.Operation directly
// This provides backward compatibility while we complete the consolidation
type Operation = operations.Operation

// ValidationError is now defined in the core package
type ValidationError = core.ValidationError
