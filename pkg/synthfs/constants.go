package synthfs

// --- Archive Format Constants ---

// ArchiveFormat defines the type of archive.
type ArchiveFormat int

const (
	// ArchiveFormatTarGz represents a tar.gz archive format.
	ArchiveFormatTarGz ArchiveFormat = iota
	// ArchiveFormatZip represents a zip archive format.
	ArchiveFormatZip
)

// --- Path State Constants ---

// PathStateType represents the type of a filesystem object in the projected state.
type PathStateType int

const (
	// PathStateUnknown represents an unknown or non-existent path.
	PathStateUnknown PathStateType = iota
	// PathStateFile represents a file.
	PathStateFile
	// PathStateDir represents a directory.
	PathStateDir
	// PathStateSymlink represents a symbolic link.
	PathStateSymlink
)

// --- Operation Status Constants ---

// OperationStatus indicates the outcome of an individual operation's execution.
type OperationStatus string

const (
	// StatusSuccess indicates the operation completed successfully.
	StatusSuccess OperationStatus = "SUCCESS"
	// StatusFailure indicates the operation failed during execution.
	StatusFailure OperationStatus = "FAILURE"
	// StatusValidation indicates the operation failed during validation.
	StatusValidation OperationStatus = "VALIDATION_FAILURE"
)

// --- Item Type Constants ---

// Item type string constants
const (
	// ItemTypeFile is the string representation of a file item.
	ItemTypeFile = "file"
	// ItemTypeDirectory is the string representation of a directory item.
	ItemTypeDirectory = "directory"
	// ItemTypeSymlink is the string representation of a symlink item.
	ItemTypeSymlink = "symlink"
	// ItemTypeArchive is the string representation of an archive item.
	ItemTypeArchive = "archive"
	// ItemTypeUnarchive is the string representation of an unarchive item.
	ItemTypeUnarchive = "unarchive"
)

// --- Operation Type Constants ---

// Operation type string constants
const (
	// OpTypeCreateFile is the string representation of a file creation operation.
	OpTypeCreateFile = "create_file"
	// OpTypeCreateDirectory is the string representation of a directory creation operation.
	OpTypeCreateDirectory = "create_directory"
	// OpTypeCreateSymlink is the string representation of a symlink creation operation.
	OpTypeCreateSymlink = "create_symlink"
	// OpTypeCreateArchive is the string representation of an archive creation operation.
	OpTypeCreateArchive = "create_archive"
	// OpTypeUnarchive is the string representation of an unarchive operation.
	OpTypeUnarchive = "unarchive"
	// OpTypeCopy is the string representation of a copy operation.
	OpTypeCopy = "copy"
	// OpTypeMove is the string representation of a move operation.
	OpTypeMove = "move"
	// OpTypeDelete is the string representation of a delete operation.
	OpTypeDelete = "delete"
)

// --- Backup Type Constants ---

// Backup type string constants
const (
	// BackupTypeFile is the string representation of a file backup.
	BackupTypeFile = "file"
	// BackupTypeDirectory is the string representation of a directory backup.
	BackupTypeDirectory = "directory"
	// BackupTypeDirectoryTree is the string representation of a directory tree backup.
	BackupTypeDirectoryTree = "directory_tree"
	// BackupTypeSymlink is the string representation of a symlink backup.
	BackupTypeSymlink = "symlink"
	// BackupTypeNone is the string representation of no backup.
	BackupTypeNone = "none"
)

// --- Default Values ---

const (
	// DefaultMaxBackupMB is the default maximum backup size in MB.
	DefaultMaxBackupMB = 10
	// DefaultFileMode is the default file permission mode (0644).
	DefaultFileMode = 0644
	// DefaultDirMode is the default directory permission mode (0755).
	DefaultDirMode = 0755
)
