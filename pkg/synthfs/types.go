package synthfs

import (
	"context"
	"io/fs"
	"time"
)

// --- Core Interface Types ---

// ReadFS is an alias for fs.FS, representing a read-only file system.
type ReadFS = fs.FS

// WriteFS defines the interface for write operations on a file system.
type WriteFS interface {
	// WriteFile writes data to a file named name.
	// If the file does not exist, WriteFile creates it with permissions perm;
	// otherwise WriteFile truncates it before writing.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// MkdirAll creates a directory named path,
	// along with any necessary parents, and returns nil,
	// or else returns an error.
	// The permission bits perm (before umask) are used for all
	// directories that MkdirAll creates.
	MkdirAll(path string, perm fs.FileMode) error

	// Remove removes the named file or (empty) directory.
	Remove(name string) error

	// RemoveAll removes path and any children it contains.
	// It removes everything it can but returns the first error
	// it encounters. If the path does not exist, RemoveAll
	// returns nil (no error).
	RemoveAll(name string) error

	// Symlink creates newname as a symbolic link to oldname.
	// On Windows, a symlink to a non-existent oldname creates a file symlink;
	// if oldname is later created as a directory, the symlink will not work.
	// If there is an error, it will be of type *LinkError.
	Symlink(oldname, newname string) error

	// Readlink returns the destination of the named symbolic link.
	// If there is an error, it will be of type *PathError.
	Readlink(name string) (string, error)

	// Rename renames (moves) oldpath to newpath.
	// If newpath already exists and is not a directory, Rename replaces it.
	// OS-specific restrictions may apply when oldpath and newpath are in different directories.
	// If there is an error, it will be of type *LinkError.
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
// It's a declarative way to define what should exist on the filesystem.
type FsItem interface {
	Path() string // Path returns the absolute path of the filesystem item.
	Type() string // Type returns a string representation of the item's type (e.g., "file", "directory").
}

// --- Operation Types ---

// OperationID is a unique identifier for an operation.
type OperationID string

// ChecksumRecord stores file checksum information for validation
type ChecksumRecord struct {
	Path         string
	MD5          string
	Size         int64
	ModTime      time.Time
	ChecksumTime time.Time
}

// OperationDesc provides a human-readable description of an operation.
type OperationDesc struct {
	Type    string                 // e.g., "create_file", "delete_directory"
	Path    string                 // Primary path affected
	Details map[string]interface{} // Additional operation-specific details
}

// BackupData stores the data needed to restore an operation's effects
type BackupData struct {
	OperationID   OperationID            `json:"operation_id"`
	BackupType    string                 `json:"backup_type"`    // "file", "directory", "none"
	OriginalPath  string                 `json:"original_path"`  // Path that was affected
	BackupContent []byte                 `json:"backup_content"` // File content backup
	BackupMode    fs.FileMode            `json:"backup_mode"`    // Original file mode
	BackupTime    time.Time              `json:"backup_time"`    // When backup was created
	SizeMB        float64                `json:"size_mb"`        // Size in MB for budget tracking
	Metadata      map[string]interface{} `json:"metadata"`       // Additional metadata
	// For "directory_tree", Metadata["items"] will be []BackedUpItem
}

// BackedUpItem represents a single file or directory within a backed-up directory tree.
// This struct will be stored in the Metadata field of the main BackupData object for the directory.
type BackedUpItem struct {
	RelativePath string      `json:"relative_path"` // Path relative to the root of the backup
	ItemType     string      `json:"item_type"`     // "file" or "directory"
	Mode         fs.FileMode `json:"mode"`          // Original file/directory mode
	Content      []byte      `json:"-"`             // File content (nil for directories); json ignored for cleaner metadata logs
	Size         int64       `json:"size"`          // File size (0 for directories if not storing their own metadata size)
	ModTime      time.Time   `json:"mod_time"`      // Original modification time
}

// BackupBudget tracks memory usage for backup operations
type BackupBudget struct {
	TotalMB     float64 `json:"total_mb"`
	RemainingMB float64 `json:"remaining_mb"`
	UsedMB      float64 `json:"used_mb"`
}

// Operation defines a single abstract filesystem operation.
type Operation interface {
	// ID returns the unique identifier of the operation.
	ID() OperationID

	// Dependencies returns a list of OperationIDs that must be successfully
	// executed before this operation can run.
	Dependencies() []OperationID

	// Conflicts returns a list of OperationIDs that cannot run concurrently
	// with this operation or that represent incompatible desired states.
	Conflicts() []OperationID

	// Execute performs the operation on the given filesystem.
	Execute(ctx context.Context, fsys FileSystem) error

	// Validate checks if the operation can be performed.
	Validate(ctx context.Context, fsys FileSystem) error

	// Rollback attempts to undo the effects of the Execute method.
	Rollback(ctx context.Context, fsys FileSystem) error

	// Describe returns a structured description of the operation.
	Describe() OperationDesc

	// GetItem returns the FsItem associated with this operation, if any.
	// This is primarily relevant for Create operations.
	// Returns nil if no item is directly associated (e.g., for Delete, Copy, Move by path).
	GetItem() FsItem

	// GetChecksum retrieves a checksum record for a file path (Phase I, Milestone 3)
	GetChecksum(path string) *ChecksumRecord

	// GetAllChecksums returns all checksum records (Phase I, Milestone 3)
	GetAllChecksums() map[string]*ChecksumRecord

	// ReverseOps generates operations that would undo this operation's effects (Phase III)
	// Returns a slice of operations that, when executed, will restore the filesystem
	// to the state it was in before this operation was executed.
	ReverseOps(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error)
}
