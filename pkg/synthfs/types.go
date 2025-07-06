package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"time"
)

// --- Core Interface Types ---

// ReadFS is an alias for fs.FS, representing a read-only file system.
type ReadFS = fs.FS

// WriteFS defines the interface for write operations on a file system.
type WriteFS interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(name string) error
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	Rename(oldpath, newpath string) error
}

// FileSystem combines read and write operations.
type FileSystem interface {
	ReadFS
	WriteFS
}

// StatFS extends ReadFS with Stat capabilities for better io/fs compatibility
type StatFS interface {
	ReadFS
	Stat(name string) (fs.FileInfo, error)
}

// FullFileSystem provides the complete filesystem interface including Stat
type FullFileSystem interface {
	FileSystem
	Stat(name string) (fs.FileInfo, error)
}

// --- FsItem Types ---

// FsItem represents a filesystem item to be created.
type FsItem interface {
	Path() string
	Type() string
}

// --- Operation Types ---

type OperationID string

type ChecksumRecord struct {
	Path         string
	MD5          string
	Size         int64
	ModTime      time.Time
	ChecksumTime time.Time
}

type OperationDesc struct {
	Type    string
	Path    string
	Details map[string]interface{}
}

type BackupData struct {
	OperationID   OperationID
	BackupType    string
	OriginalPath  string
	BackupContent []byte
	BackupMode    fs.FileMode
	BackupTime    time.Time
	SizeMB        float64
	Metadata      map[string]interface{}
}

type BackedUpItem struct {
	RelativePath string
	ItemType     string
	Mode         fs.FileMode
	Content      []byte
	Size         int64
	ModTime      time.Time
}

type BackupBudget struct {
	TotalMB     float64
	RemainingMB float64
	UsedMB      float64
}

type Operation interface {
	ID() OperationID
	Dependencies() []OperationID
	Conflicts() []OperationID
	Execute(ctx context.Context, fsys FileSystem) error
	Validate(ctx context.Context, fsys FileSystem) error
	Rollback(ctx context.Context, fsys FileSystem) error
	Describe() OperationDesc
	GetItem() FsItem
	GetChecksum(path string) *ChecksumRecord
	GetAllChecksums() map[string]*ChecksumRecord
	ReverseOps(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error)
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
