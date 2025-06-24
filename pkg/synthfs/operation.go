package synthfs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
)

// OperationID is a unique identifier for an operation.
type OperationID string

// FileSystem interface is defined in fs.go

// OperationDesc provides a human-readable description of an operation.
type OperationDesc struct {
	Type    string                 // e.g., "create_file", "delete_directory"
	Path    string                 // Primary path affected
	Details map[string]interface{} // Additional operation-specific details
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
}

// --- SimpleOperation: Basic Operation Implementation ---

// SimpleOperation provides a straightforward implementation of Operation.
// Operations are created complete and immutable - no post-creation modification.
type SimpleOperation struct {
	id           OperationID
	dependencies []OperationID
	description  OperationDesc
	item         FsItem // For Create operations
	srcPath      string // For Copy/Move operations
	dstPath      string // For Copy/Move operations
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

// --- Error Types ---

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

// DependencyError represents an error with operation dependencies.
type DependencyError struct {
	Operation    Operation
	Dependencies []OperationID
	Missing      []OperationID
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency error for operation %s: missing dependencies %v (required: %v)",
		e.Operation.ID(), e.Missing, e.Dependencies)
}

// ConflictError represents an error when operations conflict with each other.
type ConflictError struct {
	Operation Operation
	Conflicts []OperationID
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict error for operation %s: conflicts with operations %v",
		e.Operation.ID(), e.Conflicts)
}

// --- Operation Implementation Methods ---

// executeCreateFile implements file creation
func (op *SimpleOperation) executeCreateFile(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return fmt.Errorf("no file item provided for create_file operation")
	}

	fileItem, ok := op.item.(*FileItem)
	if !ok {
		return fmt.Errorf("expected FileItem for create_file operation, got %T", op.item)
	}

	return fsys.WriteFile(fileItem.Path(), fileItem.Content(), fileItem.Mode())
}

// executeCreateDirectory implements directory creation
func (op *SimpleOperation) executeCreateDirectory(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return fmt.Errorf("no directory item provided for create_directory operation")
	}

	dirItem, ok := op.item.(*DirectoryItem)
	if !ok {
		return fmt.Errorf("expected DirectoryItem for create_directory operation, got %T", op.item)
	}

	return fsys.MkdirAll(dirItem.Path(), dirItem.Mode())
}

// executeCreateSymlink implements symlink creation
func (op *SimpleOperation) executeCreateSymlink(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return fmt.Errorf("no symlink item provided for create_symlink operation")
	}

	symlinkItem, ok := op.item.(*SymlinkItem)
	if !ok {
		return fmt.Errorf("expected SymlinkItem for create_symlink operation, got %T", op.item)
	}

	return fsys.Symlink(symlinkItem.Target(), symlinkItem.Path())
}

// executeCreateArchive implements archive creation
func (op *SimpleOperation) executeCreateArchive(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return fmt.Errorf("no archive item provided for create_archive operation")
	}

	archiveItem, ok := op.item.(*ArchiveItem)
	if !ok {
		return fmt.Errorf("expected ArchiveItem for create_archive operation, got %T", op.item)
	}

	switch archiveItem.Format() {
	case ArchiveFormatTarGz:
		return op.createTarGzArchive(archiveItem, fsys)
	case ArchiveFormatZip:
		return op.createZipArchive(archiveItem, fsys)
	default:
		return fmt.Errorf("unsupported archive format: %s", archiveItem.Format())
	}
}

// createTarGzArchive creates a tar.gz archive
func (op *SimpleOperation) createTarGzArchive(archiveItem *ArchiveItem, fsys FileSystem) error {
	// Create a buffer to hold the archive data
	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close gzip writer: %v\n", closeErr)
		}
	}()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if closeErr := tarWriter.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close tar writer: %v\n", closeErr)
		}
	}()

	for _, source := range archiveItem.Sources() {
		if err := op.addToTarArchive(tarWriter, source, fsys); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", source, err)
		}
	}

	// Close writers to flush data
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Write the complete archive to the filesystem
	return fsys.WriteFile(archiveItem.Path(), buf.Bytes(), 0644)
}

// createZipArchive creates a zip archive
func (op *SimpleOperation) createZipArchive(archiveItem *ArchiveItem, fsys FileSystem) error {
	// Create a buffer to hold the archive data
	var buf bytes.Buffer

	zipWriter := zip.NewWriter(&buf)
	defer func() {
		if closeErr := zipWriter.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close zip writer: %v\n", closeErr)
		}
	}()

	for _, source := range archiveItem.Sources() {
		if err := op.addToZipArchive(zipWriter, source, fsys); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", source, err)
		}
	}

	// Close writer to flush data
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %w", err)
	}

	// Write the complete archive to the filesystem
	return fsys.WriteFile(archiveItem.Path(), buf.Bytes(), 0644)
}

// addToTarArchive adds a file or directory to a tar archive
func (op *SimpleOperation) addToTarArchive(tarWriter *tar.Writer, sourcePath string, fsys FileSystem) error {
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation needed for archiving")
	}

	// Get file info
	info, err := fullFS.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header for %s: %w", sourcePath, err)
	}

	// Use relative path in archive
	header.Name = strings.TrimPrefix(sourcePath, "./")

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for %s: %w", sourcePath, err)
	}

	// If it's a file, write content
	if !info.IsDir() {
		file, err := fsys.Open(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", sourcePath, err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Printf("Warning: failed to close file %s: %v\n", sourcePath, closeErr)
			}
		}()

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to write file content for %s: %w", sourcePath, err)
		}
	}

	return nil
}

// addToZipArchive adds a file or directory to a zip archive
func (op *SimpleOperation) addToZipArchive(zipWriter *zip.Writer, sourcePath string, fsys FileSystem) error {
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation needed for archiving")
	}

	// Get file info
	info, err := fullFS.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	// Use relative path in archive
	archivePath := strings.TrimPrefix(sourcePath, "./")

	// If it's a directory, create directory entry
	if info.IsDir() {
		if !strings.HasSuffix(archivePath, "/") {
			archivePath += "/"
		}
		_, err := zipWriter.Create(archivePath)
		return err
	}

	// Create file entry
	writer, err := zipWriter.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create zip entry for %s: %w", sourcePath, err)
	}

	// Open and copy file content
	file, err := fsys.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file %s: %v\n", sourcePath, closeErr)
		}
	}()

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("failed to write file content for %s: %w", sourcePath, err)
	}

	return nil
}

// executeCopy implements file/directory copying
func (op *SimpleOperation) executeCopy(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" || op.dstPath == "" {
		return fmt.Errorf("source or destination path not set for copy operation")
	}

	// For now, implement simple file copy - directory copy is more complex
	// First check if source is a file
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation needed for copy")
	}

	srcInfo, err := fullFS.Stat(op.srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", op.srcPath, err)
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("directory copying not yet implemented")
	}

	// Read source file content
	file, err := fsys.Open(op.srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", op.srcPath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close source file %s: %v\n", op.srcPath, closeErr)
		}
	}()

	content := make([]byte, srcInfo.Size())
	_, err = file.Read(content)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", op.srcPath, err)
	}

	// Write to destination
	return fsys.WriteFile(op.dstPath, content, srcInfo.Mode())
}

// executeMove implements file/directory moving
func (op *SimpleOperation) executeMove(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" || op.dstPath == "" {
		return fmt.Errorf("source or destination path not set for move operation")
	}

	return fsys.Rename(op.srcPath, op.dstPath)
}

// executeDelete implements file/directory deletion
func (op *SimpleOperation) executeDelete(ctx context.Context, fsys FileSystem) error {
	path := op.description.Path

	// Check if it's a directory or file
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		// Fallback: try Remove first, then RemoveAll
		err := fsys.Remove(path)
		if err != nil {
			return fsys.RemoveAll(path)
		}
		return nil
	}

	info, err := fullFS.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path for deletion %s: %w", path, err)
	}

	if info.IsDir() {
		return fsys.RemoveAll(path)
	}
	return fsys.Remove(path)
}

// validateCreateFile validates file creation
func (op *SimpleOperation) validateCreateFile(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{Operation: op, Reason: "no file item provided"}
	}

	fileItem, ok := op.item.(*FileItem)
	if !ok {
		return &ValidationError{Operation: op, Reason: fmt.Sprintf("expected FileItem, got %T", op.item)}
	}

	if fileItem.Path() == "" {
		return &ValidationError{Operation: op, Reason: "file path cannot be empty"}
	}

	return nil
}

// validateCreateDirectory validates directory creation
func (op *SimpleOperation) validateCreateDirectory(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{Operation: op, Reason: "no directory item provided"}
	}

	dirItem, ok := op.item.(*DirectoryItem)
	if !ok {
		return &ValidationError{Operation: op, Reason: fmt.Sprintf("expected DirectoryItem, got %T", op.item)}
	}

	if dirItem.Path() == "" {
		return &ValidationError{Operation: op, Reason: "directory path cannot be empty"}
	}

	return nil
}

// validateCreateSymlink validates symlink creation
func (op *SimpleOperation) validateCreateSymlink(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{Operation: op, Reason: "no symlink item provided"}
	}

	symlinkItem, ok := op.item.(*SymlinkItem)
	if !ok {
		return &ValidationError{Operation: op, Reason: fmt.Sprintf("expected SymlinkItem, got %T", op.item)}
	}

	if symlinkItem.Path() == "" {
		return &ValidationError{Operation: op, Reason: "symlink path cannot be empty"}
	}

	if symlinkItem.Target() == "" {
		return &ValidationError{Operation: op, Reason: "symlink target cannot be empty"}
	}

	return nil
}

// validateCreateArchive validates archive creation
func (op *SimpleOperation) validateCreateArchive(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{Operation: op, Reason: "no archive item provided"}
	}

	archiveItem, ok := op.item.(*ArchiveItem)
	if !ok {
		return &ValidationError{Operation: op, Reason: fmt.Sprintf("expected ArchiveItem, got %T", op.item)}
	}

	if archiveItem.Path() == "" {
		return &ValidationError{Operation: op, Reason: "archive path cannot be empty"}
	}

	if len(archiveItem.Sources()) == 0 {
		return &ValidationError{Operation: op, Reason: "archive must have at least one source"}
	}

	return nil
}

// validateCopy validates copy operation
func (op *SimpleOperation) validateCopy(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" {
		return &ValidationError{Operation: op, Reason: "copy source path cannot be empty"}
	}

	if op.dstPath == "" {
		return &ValidationError{Operation: op, Reason: "copy destination path cannot be empty"}
	}

	// For copy operations, we don't check if source exists at validation time
	// because it might be created by an earlier operation in the same batch.
	// We'll check existence at execution time instead.

	return nil
}

// validateMove validates move operation
func (op *SimpleOperation) validateMove(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" {
		return &ValidationError{Operation: op, Reason: "move source path cannot be empty"}
	}

	if op.dstPath == "" {
		return &ValidationError{Operation: op, Reason: "move destination path cannot be empty"}
	}

	// For move operations, we don't check if source exists at validation time
	// because it might be created by an earlier operation in the same batch.
	// We'll check existence at execution time instead.

	return nil
}

// validateDelete validates delete operation
func (op *SimpleOperation) validateDelete(ctx context.Context, fsys FileSystem) error {
	path := op.description.Path
	if path == "" {
		return &ValidationError{Operation: op, Reason: "delete path cannot be empty"}
	}

	// For delete operations, we don't check if target exists at validation time
	// because it might be created by an earlier operation in the same batch.
	// We'll check existence at execution time instead.

	return nil
}

// rollbackCreate removes what was created
func (op *SimpleOperation) rollbackCreate(ctx context.Context, fsys FileSystem) error {
	return fsys.Remove(op.description.Path)
}

// rollbackCopy removes the destination of the copy
func (op *SimpleOperation) rollbackCopy(ctx context.Context, fsys FileSystem) error {
	return fsys.Remove(op.dstPath)
}

// rollbackMove moves the file back to its original location
func (op *SimpleOperation) rollbackMove(ctx context.Context, fsys FileSystem) error {
	return fsys.Rename(op.dstPath, op.srcPath)
}
