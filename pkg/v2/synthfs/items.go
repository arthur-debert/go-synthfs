package synthfs

import "io/fs"

// FsItem represents a filesystem item to be created.
// It's a declarative way to define what should exist on the filesystem.
type FsItem interface {
	Path() string // Path returns the absolute path of the filesystem item.
	Type() string // Type returns a string representation of the item's type (e.g., "file", "directory").
}

// --- FileItem ---

// FileItem represents a file to be created.
type FileItem struct {
	path    string
	content []byte
	mode    fs.FileMode
}

// NewFile creates a new FileItem builder.
// Path is the absolute path to the file.
func NewFile(path string) *FileItem {
	return &FileItem{
		path: path,
		mode: 0644, // Default mode
	}
}

// Path returns the file's path.
func (fi *FileItem) Path() string {
	return fi.path
}

// Type returns "file".
func (fi *FileItem) Type() string {
	return "file"
}

// WithContent sets the content for the file.
func (fi *FileItem) WithContent(content []byte) *FileItem {
	fi.content = content
	return fi
}

// Content returns the file's content.
func (fi *FileItem) Content() []byte {
	return fi.content
}

// WithMode sets the file mode (permissions).
func (fi *FileItem) WithMode(mode fs.FileMode) *FileItem {
	fi.mode = mode
	return fi
}

// Mode returns the file's mode.
func (fi *FileItem) Mode() fs.FileMode {
	return fi.mode
}

// --- DirectoryItem ---

// DirectoryItem represents a directory to be created.
type DirectoryItem struct {
	path string
	mode fs.FileMode
}

// NewDirectory creates a new DirectoryItem builder.
// Path is the absolute path to the directory.
func NewDirectory(path string) *DirectoryItem {
	return &DirectoryItem{
		path: path,
		mode: 0755, // Default mode
	}
}

// Path returns the directory's path.
func (di *DirectoryItem) Path() string {
	return di.path
}

// Type returns "directory".
func (di *DirectoryItem) Type() string {
	return "directory"
}

// WithMode sets the directory mode (permissions).
func (di *DirectoryItem) WithMode(mode fs.FileMode) *DirectoryItem {
	di.mode = mode
	return di
}

// Mode returns the directory's mode.
func (di *DirectoryItem) Mode() fs.FileMode {
	return di.mode
}

// --- SymlinkItem ---

// SymlinkItem represents a symbolic link to be created.
type SymlinkItem struct {
	path   string
	target string
}

// NewSymlink creates a new SymlinkItem builder.
// Path is the absolute path to the symlink.
// Target is the path the symlink should point to.
func NewSymlink(path string, target string) *SymlinkItem {
	return &SymlinkItem{
		path:   path,
		target: target,
	}
}

// Path returns the symlink's path.
func (si *SymlinkItem) Path() string {
	return si.path
}

// Type returns "symlink".
func (si *SymlinkItem) Type() string {
	return "symlink"
}

// Target returns the symlink's target path.
func (si *SymlinkItem) Target() string {
	return si.target
}

// --- ArchiveItem ---

// ArchiveFormat defines the type of archive.
type ArchiveFormat int

const (
	ArchiveFormatTarGz ArchiveFormat = iota
	ArchiveFormatZip
)

// String returns the string representation of the ArchiveFormat.
// This implements the fmt.Stringer interface.
func (af ArchiveFormat) String() string {
	switch af {
	case ArchiveFormatTarGz:
		return "tar.gz"
	case ArchiveFormatZip:
		return "zip"
	default:
		return "unknown"
	}
}

// ArchiveItem represents an archive to be created.
type ArchiveItem struct {
	path    string
	format  ArchiveFormat
	sources []string
}

// NewArchive creates a new ArchiveItem builder.
// Path is the absolute path to the archive file.
// Format is the archive format (e.g., TarGz, Zip).
// Sources are the paths to the files/directories to be included in the archive.
func NewArchive(path string, format ArchiveFormat, sources []string) *ArchiveItem {
	return &ArchiveItem{
		path:    path,
		format:  format,
		sources: sources,
	}
}

// Path returns the archive's path.
func (ai *ArchiveItem) Path() string {
	return ai.path
}

// Type returns "archive".
func (ai *ArchiveItem) Type() string {
	return "archive"
}

// Format returns the archive's format.
func (ai *ArchiveItem) Format() ArchiveFormat {
	return ai.format
}

// Sources returns the list of source paths for the archive.
func (ai *ArchiveItem) Sources() []string {
	return ai.sources
}

// WithSources sets the sources for the archive.
// This allows modifying sources after initial creation if needed.
func (ai *ArchiveItem) WithSources(sources []string) *ArchiveItem {
	ai.sources = sources
	return ai
}
