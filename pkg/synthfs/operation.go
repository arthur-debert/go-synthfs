package synthfs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// Operation types are now defined in types.go

// ConsumeBackup reduces the remaining budget by the specified amount
func (bb *BackupBudget) ConsumeBackup(sizeMB float64) error {
	if sizeMB > bb.RemainingMB {
		return fmt.Errorf("backup size %.2fMB exceeds remaining budget %.2fMB", sizeMB, bb.RemainingMB)
	}
	bb.RemainingMB -= sizeMB
	bb.UsedMB += sizeMB
	return nil
}

// RestoreBackup increases the remaining budget by the specified amount
func (bb *BackupBudget) RestoreBackup(sizeMB float64) {
	bb.RemainingMB += sizeMB
	bb.UsedMB -= sizeMB
	if bb.UsedMB < 0 {
		bb.UsedMB = 0
	}
	if bb.RemainingMB > bb.TotalMB {
		bb.RemainingMB = bb.TotalMB
	}
}

// Operation interface is defined in types.go

// --- SimpleOperation: Basic Operation Implementation ---

// SimpleOperation provides a straightforward implementation of Operation.
// Operations are created complete and immutable - no post-creation modification.
type SimpleOperation struct {
	id           OperationID
	dependencies []OperationID
	description  OperationDesc
	item         FsItem                     // For Create operations
	srcPath      string                     // For Copy/Move operations
	dstPath      string                     // For Copy/Move operations
	checksums    map[string]*ChecksumRecord // Phase I, Milestone 3: Store file checksums
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
		checksums:    make(map[string]*ChecksumRecord), // Phase I, Milestone 3: Initialize checksum storage
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

// GetSrcPath returns the source path for copy/move operations.
func (op *SimpleOperation) GetSrcPath() string {
	return op.srcPath
}

// GetDstPath returns the destination path for copy/move operations.
func (op *SimpleOperation) GetDstPath() string {
	return op.dstPath
}

// SetChecksum stores a checksum record for a file path (Phase I, Milestone 3)
func (op *SimpleOperation) SetChecksum(path string, checksum *ChecksumRecord) {
	if op.checksums == nil {
		op.checksums = make(map[string]*ChecksumRecord)
	}
	op.checksums[path] = checksum
}

// GetChecksum retrieves a checksum record for a file path (Phase I, Milestone 3)
func (op *SimpleOperation) GetChecksum(path string) *ChecksumRecord {
	if op.checksums == nil {
		return nil
	}
	return op.checksums[path]
}

// GetAllChecksums returns all checksum records (Phase I, Milestone 3)
func (op *SimpleOperation) GetAllChecksums() map[string]*ChecksumRecord {
	return op.checksums
}

// verifyChecksums verifies all stored checksums against current file state (Phase I, Milestone 4)
func (op *SimpleOperation) verifyChecksums(ctx context.Context, fsys FileSystem) error {
	if len(op.checksums) == 0 {
		return nil // No checksums to verify
	}

	// Check if filesystem supports Stat operation
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		// If filesystem doesn't support Stat, we cannot compute a checksum.
		// Log a warning and skip verification.
		Logger().Warn().
			Str("op_id", string(op.ID())).
			Msg("skipping checksum verification: filesystem does not support Stat")
		return nil
	}

	for path, expectedChecksum := range op.checksums {
		// Re-compute the checksum for the current file state
		currentChecksum, err := ComputeFileChecksum(fullFS, path)
		if err != nil {
			return fmt.Errorf("checksum verification failed for %s: could not compute current checksum: %w", path, err)
		}

		// It's possible for a file to be replaced by a directory
		if currentChecksum == nil && expectedChecksum != nil {
			return fmt.Errorf("checksum verification failed for %s: expected a file but found a directory", path)
		}

		// Compare the MD5 hashes
		if currentChecksum.MD5 != expectedChecksum.MD5 {
			return fmt.Errorf("checksum verification failed for %s: file content has changed. Expected MD5: %s, got: %s",
				path, expectedChecksum.MD5, currentChecksum.MD5)
		}

		// Optional: We could still log if modtime/size differ but hash is same, but for now hash equality is sufficient.
		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("path", path).
			Str("md5", currentChecksum.MD5).
			Msg("checksum verification passed")
	}

	return nil
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
	case "unarchive":
		return op.executeUnarchive(ctx, fsys)
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
	case "unarchive":
		return op.validateUnarchive(ctx, fsys)
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
	case "unarchive":
		// For unarchive operations, rollback means removing extracted files
		return op.rollbackUnarchive(ctx, fsys)
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

// ReverseOps generates operations that would undo this operation's effects (Phase III)
func (op *SimpleOperation) ReverseOps(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	switch op.description.Type {
	case "create_file":
		return op.reverseCreateFile(ctx, fsys, budget)
	case "create_directory":
		return op.reverseCreateDirectory(ctx, fsys, budget)
	case "create_symlink":
		return op.reverseCreateSymlink(ctx, fsys, budget)
	case "create_archive":
		return op.reverseCreateArchive(ctx, fsys, budget)
	case "unarchive":
		return op.reverseUnarchive(ctx, fsys, budget)
	case "copy":
		return op.reverseCopy(ctx, fsys, budget)
	case "move":
		return op.reverseMove(ctx, fsys, budget)
	case "delete":
		return op.reverseDelete(ctx, fsys, budget)
	default:
		return nil, nil, fmt.Errorf("unknown operation type for reverse ops: %s", op.description.Type)
	}
}

// Error types are now defined in errors.go

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

	// Phase I, Milestone 4: Verify checksums before execution
	if err := op.verifyChecksums(ctx, fsys); err != nil {
		return fmt.Errorf("archive creation failed checksum verification: %w", err)
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

// executeUnarchive implements archive extraction
func (op *SimpleOperation) executeUnarchive(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return fmt.Errorf("no unarchive item provided for unarchive operation")
	}

	unarchiveItem, ok := op.item.(*UnarchiveItem)
	if !ok {
		return fmt.Errorf("expected UnarchiveItem for unarchive operation, got %T", op.item)
	}

	// Determine archive format from file extension
	archivePath := unarchiveItem.ArchivePath()
	var format ArchiveFormat
	if strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz") || strings.HasSuffix(strings.ToLower(archivePath), ".tgz") {
		format = ArchiveFormatTarGz
	} else if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		format = ArchiveFormatZip
	} else {
		return fmt.Errorf("unsupported archive format for file: %s", archivePath)
	}

	switch format {
	case ArchiveFormatTarGz:
		return op.extractTarGzArchive(unarchiveItem, fsys)
	case ArchiveFormatZip:
		return op.extractZipArchive(unarchiveItem, fsys)
	default:
		return fmt.Errorf("unsupported archive format: %s", format.String())
	}
}

// extractTarGzArchive extracts a tar.gz archive
func (op *SimpleOperation) extractTarGzArchive(unarchiveItem *UnarchiveItem, fsys FileSystem) error {
	// Open archive file
	file, err := fsys.Open(unarchiveItem.ArchivePath())
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", unarchiveItem.ArchivePath(), err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close archive file: %v\n", closeErr)
		}
	}()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close gzip reader: %v\n", closeErr)
		}
	}()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Check if file matches patterns (if any)
		if !op.matchesPatterns(header.Name, unarchiveItem.Patterns()) {
			continue
		}

		// Determine extraction path
		extractPath := filepath.Join(unarchiveItem.ExtractPath(), header.Name)

		// Ensure extract path is safe (prevent directory traversal)
		if !strings.HasPrefix(filepath.Clean(extractPath), filepath.Clean(unarchiveItem.ExtractPath())) {
			return fmt.Errorf("unsafe path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := fsys.MkdirAll(extractPath, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", extractPath, err)
			}

		case tar.TypeReg:
			// Create file
			if err := op.extractFileFromTar(tarReader, extractPath, header.FileInfo().Mode(), unarchiveItem.Overwrite(), fsys); err != nil {
				return fmt.Errorf("failed to extract file %s: %w", extractPath, err)
			}

		case tar.TypeLink, tar.TypeSymlink:
			// Skip symlinks and hard links for now
			fmt.Printf("Warning: skipping link %s\n", header.Name)
			continue

		default:
			fmt.Printf("Warning: skipping unsupported file type %c for %s\n", header.Typeflag, header.Name)
		}
	}

	return nil
}

// extractZipArchive extracts a zip archive
func (op *SimpleOperation) extractZipArchive(unarchiveItem *UnarchiveItem, fsys FileSystem) error {
	// Read archive file content
	file, err := fsys.Open(unarchiveItem.ArchivePath())
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", unarchiveItem.ArchivePath(), err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close archive file: %v\n", closeErr)
		}
	}()

	// Get file info for size
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation needed for zip extraction")
	}

	info, err := fullFS.Stat(unarchiveItem.ArchivePath())
	if err != nil {
		return fmt.Errorf("failed to stat archive file: %w", err)
	}

	// Read all content into memory (required for zip.NewReader)
	content := make([]byte, info.Size())
	_, err = file.Read(content)
	if err != nil {
		return fmt.Errorf("failed to read archive content: %w", err)
	}

	// Create zip reader
	zipReader, err := zip.NewReader(bytes.NewReader(content), info.Size())
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Extract files
	for _, f := range zipReader.File {
		// Check if file matches patterns (if any)
		if !op.matchesPatterns(f.Name, unarchiveItem.Patterns()) {
			continue
		}

		// Determine extraction path
		extractPath := filepath.Join(unarchiveItem.ExtractPath(), f.Name)

		// Ensure extract path is safe (prevent directory traversal)
		if !strings.HasPrefix(filepath.Clean(extractPath), filepath.Clean(unarchiveItem.ExtractPath())) {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			// Create directory
			if err := fsys.MkdirAll(extractPath, f.FileInfo().Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", extractPath, err)
			}
		} else {
			// Extract file
			if err := op.extractFileFromZip(f, extractPath, unarchiveItem.Overwrite(), fsys); err != nil {
				return fmt.Errorf("failed to extract file %s: %w", extractPath, err)
			}
		}
	}

	return nil
}

// extractFileFromTar extracts a single file from a tar archive
func (op *SimpleOperation) extractFileFromTar(tarReader *tar.Reader, extractPath string, mode fs.FileMode, overwrite bool, fsys FileSystem) error {
	// Check if file already exists
	if !overwrite {
		if fullFS, ok := fsys.(FullFileSystem); ok {
			if _, err := fullFS.Stat(extractPath); err == nil {
				return fmt.Errorf("file already exists: %s", extractPath)
			}
		}
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(extractPath)
	if err := fsys.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	// Read file content
	content, err := io.ReadAll(tarReader)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	// Write file
	return fsys.WriteFile(extractPath, content, mode)
}

// extractFileFromZip extracts a single file from a zip archive
func (op *SimpleOperation) extractFileFromZip(f *zip.File, extractPath string, overwrite bool, fsys FileSystem) error {
	// Check if file already exists
	if !overwrite {
		if fullFS, ok := fsys.(FullFileSystem); ok {
			if _, err := fullFS.Stat(extractPath); err == nil {
				return fmt.Errorf("file already exists: %s", extractPath)
			}
		}
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(extractPath)
	if err := fsys.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	// Open file in archive
	reader, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in archive: %w", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close zip file reader: %v\n", closeErr)
		}
	}()

	// Read file content
	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	// Write file
	return fsys.WriteFile(extractPath, content, f.FileInfo().Mode())
}

// matchesPatterns checks if a file path matches any of the given patterns
func (op *SimpleOperation) matchesPatterns(filePath string, patterns []string) bool {
	// If no patterns specified, match all files
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		// Use filepath.Match for simple patterns
		if matched, err := filepath.Match(pattern, filePath); err == nil && matched {
			return true
		}

		// Also handle directory-based patterns like "docs/**"
		if strings.HasSuffix(pattern, "/**") {
			dirPattern := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(filePath, dirPattern+"/") || filePath == dirPattern {
				return true
			}
		}

		// Handle patterns with directory separators
		if strings.Contains(pattern, "/") {
			if matched, err := filepath.Match(pattern, filePath); err == nil && matched {
				return true
			}
		}
	}

	return false
}

// executeCopy implements file/directory copying
func (op *SimpleOperation) executeCopy(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" || op.dstPath == "" {
		return fmt.Errorf("source or destination path not set for copy operation")
	}

	// Phase I, Milestone 4: Verify checksums before execution
	if err := op.verifyChecksums(ctx, fsys); err != nil {
		return fmt.Errorf("copy operation failed checksum verification: %w", err)
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

	// Phase I, Milestone 4: Verify checksums before execution
	if err := op.verifyChecksums(ctx, fsys); err != nil {
		return fmt.Errorf("move operation failed checksum verification: %w", err)
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

	// Phase I, Milestone 1: Source existence validation
	// Check if all source files/directories exist at validation time
	if fullFS, ok := fsys.(FullFileSystem); ok {
		for _, source := range archiveItem.Sources() {
			if _, err := fullFS.Stat(source); err != nil {
				return &ValidationError{
					Operation: op,
					Reason:    fmt.Sprintf("archive source does not exist: %s", source),
					Cause:     err,
				}
			}
		}
	}

	return nil
}

// validateUnarchive validates unarchive operation
func (op *SimpleOperation) validateUnarchive(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{Operation: op, Reason: "no unarchive item provided"}
	}

	unarchiveItem, ok := op.item.(*UnarchiveItem)
	if !ok {
		return &ValidationError{Operation: op, Reason: fmt.Sprintf("expected UnarchiveItem, got %T", op.item)}
	}

	if unarchiveItem.ArchivePath() == "" {
		return &ValidationError{Operation: op, Reason: "archive path cannot be empty"}
	}

	if unarchiveItem.ExtractPath() == "" {
		return &ValidationError{Operation: op, Reason: "extract path cannot be empty"}
	}

	// Validate archive format is supported
	archivePath := unarchiveItem.ArchivePath()
	if !strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz") &&
		!strings.HasSuffix(strings.ToLower(archivePath), ".tgz") &&
		!strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return &ValidationError{Operation: op, Reason: "unsupported archive format (supported: .tar.gz, .tgz, .zip)"}
	}

	// Phase I, Milestone 1: Source existence validation
	// Check if archive file exists at validation time
	if fullFS, ok := fsys.(FullFileSystem); ok {
		if _, err := fullFS.Stat(archivePath); err != nil {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("archive file does not exist: %s", archivePath),
				Cause:     err,
			}
		}
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

	// Phase I, Milestone 1: Source existence validation
	// Check if source file/directory exists at validation time
	if fullFS, ok := fsys.(FullFileSystem); ok {
		if _, err := fullFS.Stat(op.srcPath); err != nil {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("copy source does not exist: %s", op.srcPath),
				Cause:     err,
			}
		}
	}

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

	// Phase I, Milestone 1: Source existence validation
	// Check if source file/directory exists at validation time
	if fullFS, ok := fsys.(FullFileSystem); ok {
		if _, err := fullFS.Stat(op.srcPath); err != nil {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("move source does not exist: %s", op.srcPath),
				Cause:     err,
			}
		}
	}

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

// rollbackUnarchive removes extracted files (this is complex and potentially dangerous)
func (op *SimpleOperation) rollbackUnarchive(ctx context.Context, fsys FileSystem) error {
	// For safety, we don't automatically remove extracted files as it could be destructive
	// A proper implementation would need to track what was extracted during the operation
	return fmt.Errorf("rollback of unarchive operations is not automatically supported for safety reasons")
}

// Helper function to read file content
func readFileContent(fsys FullFileSystem, path string) ([]byte, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			Logger().Warn().Err(closeErr).Str("path", path).Msg("failed to close file after reading content for backup")
		}
	}()
	return io.ReadAll(file)
}

// --- Phase III: Reverse Operation Implementations ---

// reverseCreateFile generates delete operation to undo file creation
func (op *SimpleOperation) reverseCreateFile(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// Simple case: just delete the file that was created
	reverseOp := NewSimpleOperation(
		OperationID("reverse_"+string(op.ID())),
		"delete",
		op.description.Path,
	)

	// No backup needed for create operations - we just delete what was created
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.description.Path,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "delete_created_file"},
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseCreateDirectory generates delete operation to undo directory creation
func (op *SimpleOperation) reverseCreateDirectory(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// Simple case: just delete the directory that was created
	reverseOp := NewSimpleOperation(
		OperationID("reverse_"+string(op.ID())),
		"delete",
		op.description.Path,
	)

	// No backup needed for create operations - we just delete what was created
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.description.Path,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "delete_created_directory"},
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseCreateSymlink generates delete operation to undo symlink creation
func (op *SimpleOperation) reverseCreateSymlink(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// Simple case: just delete the symlink that was created
	reverseOp := NewSimpleOperation(
		OperationID("reverse_"+string(op.ID())),
		"delete",
		op.description.Path,
	)

	// No backup needed for create operations - we just delete what was created
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.description.Path,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "delete_created_symlink"},
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseCreateArchive generates delete operation to undo archive creation
func (op *SimpleOperation) reverseCreateArchive(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// Simple case: just delete the archive that was created
	reverseOp := NewSimpleOperation(
		OperationID("reverse_"+string(op.ID())),
		"delete",
		op.description.Path,
	)

	// No backup needed for create operations - we just delete what was created
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.description.Path,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "delete_created_archive"},
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseUnarchive generates delete operations to undo file extraction
func (op *SimpleOperation) reverseUnarchive(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// For unarchive, we need to delete the extraction directory
	// This is simplified - a full implementation might track individual extracted files
	var reverseOps []Operation

	if item := op.GetItem(); item != nil {
		if unarchiveItem, ok := item.(*UnarchiveItem); ok {
			// Delete the extraction directory
			reverseOp := NewSimpleOperation(
				OperationID("reverse_"+string(op.ID())),
				"delete",
				unarchiveItem.ExtractPath(),
			)
			reverseOps = append(reverseOps, reverseOp)
		}
	}

	// No backup needed for unarchive operations - we just delete what was extracted
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.description.Path,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "delete_extracted_files"},
	}

	return reverseOps, backupData, nil
}

// reverseCopy generates delete operation to undo copy
func (op *SimpleOperation) reverseCopy(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// For copy, we just delete the destination file
	reverseOp := NewSimpleOperation(
		OperationID("reverse_"+string(op.ID())),
		"delete",
		op.dstPath,
	)

	// No backup needed for copy operations - we just delete the copy
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.dstPath,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "delete_copied_file"},
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseMove generates move operation to undo move
func (op *SimpleOperation) reverseMove(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// For move, we move the file back to its original location
	reverseOp := NewSimpleOperation(
		OperationID("reverse_"+string(op.ID())),
		"move",
		op.dstPath,
	)
	reverseOp.SetPaths(op.dstPath, op.srcPath)
	reverseOp.SetDescriptionDetail("destination", op.srcPath)

	// No backup needed for move operations - we just move it back
	backupData := &BackupData{
		OperationID:   op.ID(),
		BackupType:    "none",
		OriginalPath:  op.srcPath,
		BackupContent: nil,
		BackupTime:    time.Now(),
		SizeMB:        0,
		Metadata:      map[string]interface{}{"reverse_type": "move_back", "original_src": op.srcPath, "original_dst": op.dstPath},
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseDelete generates create operation to undo delete (with budget-aware backup)
func (op *SimpleOperation) reverseDelete(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	path := op.description.Path

	// Check if the filesystem supports Stat to determine file type and size
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		return nil, nil, fmt.Errorf("reverse delete requires filesystem with Stat support")
	}

	// Get file info to determine type and size
	info, err := fullFS.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot backup file for delete operation: %w", err)
	}

	var reverseOps []Operation
	var backupData *BackupData
	// err is already declared from fullFS.Stat(path) and is nil if we reached here.
	// It will be updated by walkAndBackup or file processing errors.

	if info.IsDir() {
		backedUpItems := make([]BackedUpItem, 0)
		var totalBackedUpSize int64 // Accumulates byte size of backed-up files

		// Add the root directory itself to backedUpItems first
		// This ensures the directory itself is always listed, even if empty or if content backup fails.
		backedUpItems = append(backedUpItems, BackedUpItem{
			RelativePath: ".",
			ItemType:     "directory",
			Mode:         info.Mode(),
			ModTime:      info.ModTime(),
			Size:         0,
		})

		// Helper function for recursive backup
		// Returns the first budget error encountered, or a more critical FS error.
		var walkAndBackup func(currentPath string, relativePath string) (firstBudgetOrFSError error)
		walkAndBackup = func(currentPath string, relativePath string) (firstBudgetOrFSError error) {
			entries, readDirErr := fs.ReadDir(fullFS, currentPath)
			if readDirErr != nil {
				return fmt.Errorf("failed to read directory %s for backup: %w", currentPath, readDirErr)
			}

			for _, entry := range entries {
				entryInfo, entryInfoErr := entry.Info()
				if entryInfoErr != nil {
					return fmt.Errorf("failed to get info for entry %s in %s: %w", entry.Name(), currentPath, entryInfoErr)
				}

				entryRelativePath := filepath.Join(relativePath, entryInfo.Name())

				if entryInfo.IsDir() {
					backedUpItems = append(backedUpItems, BackedUpItem{
						RelativePath: entryRelativePath,
						ItemType:     "directory",
						Mode:         entryInfo.Mode(),
						ModTime:      entryInfo.ModTime(),
					})
					deeperErr := walkAndBackup(filepath.Join(currentPath, entryInfo.Name()), entryRelativePath)
					if deeperErr != nil {
						// Prioritize hard FS errors. If it's a budget error, only store the first one encountered.
						if !strings.Contains(deeperErr.Error(), "budget exceeded") {
							return deeperErr // Propagate hard FS errors immediately
						}
						if firstBudgetOrFSError == nil {
							firstBudgetOrFSError = deeperErr
						}
					}
				} else { // File
					fileContent, readErr := readFileContent(fullFS, filepath.Join(currentPath, entryInfo.Name()))
					if readErr != nil {
						return fmt.Errorf("failed to read file %s for backup: %w", entryRelativePath, readErr)
					}

					fileSizeBytes := entryInfo.Size()
					fileSizeMB := float64(fileSizeBytes) / (1024 * 1024)

					canAddFile := true
					if budget != nil {
						if budgetErr := budget.ConsumeBackup(fileSizeMB); budgetErr != nil {
							if firstBudgetOrFSError == nil {
								firstBudgetOrFSError = fmt.Errorf("budget exceeded for file %s (%.2fMB): %w", entryRelativePath, fileSizeMB, budgetErr)
							}
							canAddFile = false // Do not add this file if budget exceeded
						}
					}

					if canAddFile {
						backedUpItems = append(backedUpItems, BackedUpItem{
							RelativePath: entryRelativePath,
							ItemType:     "file",
							Mode:         entryInfo.Mode(),
							Content:      fileContent,
							Size:         fileSizeBytes,
							ModTime:      entryInfo.ModTime(),
						})
						totalBackedUpSize += fileSizeBytes
					}
				}
			}
			return firstBudgetOrFSError
		}

		walkErr := walkAndBackup(path, ".")
		if walkErr != nil {
			err = walkErr // Update the main error for reverseDelete
		}

		totalBackedUpSizeMB := float64(totalBackedUpSize) / (1024 * 1024)
		backupData = &BackupData{
			OperationID:   op.ID(),
			BackupType:    "directory_tree",
			OriginalPath:  path,
			BackupContent: nil,
			BackupMode:    info.Mode(),
			BackupTime:    time.Now(),
			SizeMB:        totalBackedUpSizeMB,
			Metadata: map[string]interface{}{
				"items":        backedUpItems,
				"reverse_type": "recreate_directory_tree",
			},
		}
	} else {
		// Regular file - backup content
		sizeMB := float64(info.Size()) / (1024 * 1024)

		if budget != nil {
			if budgetErr := budget.ConsumeBackup(sizeMB); budgetErr != nil {
				return nil, nil, fmt.Errorf("cannot backup file '%s' (%.2fMB): %w", path, sizeMB, budgetErr)
			}
		}

		content, fileReadErr := readFileContent(fullFS, path)
		if fileReadErr != nil {
			if budget != nil {
				budget.RestoreBackup(sizeMB)
			}
			return nil, nil, fmt.Errorf("cannot read file content for backup for %s: %w", path, fileReadErr)
		}

		reverseOp := NewSimpleOperation(
			OperationID("reverse_"+string(op.ID())),
			"create_file",
			path,
		)
		fileItem := NewFile(path).WithContent(content).WithMode(info.Mode())
		reverseOp.SetItem(fileItem)
		reverseOps = append(reverseOps, reverseOp)

		backupData = &BackupData{
			OperationID:   op.ID(),
			BackupType:    "file",
			OriginalPath:  path,
			BackupContent: content,
			BackupMode:    info.Mode(),
			BackupTime:    time.Now(),
			SizeMB:        sizeMB,
			Metadata:      map[string]interface{}{"reverse_type": "recreate_file"},
		}
	}

	// Generate reverse operations from backedUpItems if it's a directory_tree backup
	if backupData != nil && backupData.BackupType == "directory_tree" {
		if items, ok := backupData.Metadata["items"].([]BackedUpItem); ok {
			generatedOps := make([]Operation, 0, len(items))
			for i, item := range items {
				var singleRevOp Operation
				revOpItemID := OperationID(fmt.Sprintf("%s_item_%d", op.ID(), i))
				itemAbsPath := filepath.Join(path, item.RelativePath)
				if item.RelativePath == "." {
					itemAbsPath = path
				}

				if item.ItemType == "directory" {
					sOp := NewSimpleOperation(revOpItemID, "create_directory", itemAbsPath)
					dirItem := NewDirectory(itemAbsPath).WithMode(item.Mode)
					sOp.SetItem(dirItem)
					singleRevOp = sOp
				} else {
					sOp := NewSimpleOperation(revOpItemID, "create_file", itemAbsPath)
					fileItem := NewFile(itemAbsPath).WithContent(item.Content).WithMode(item.Mode)
					sOp.SetItem(fileItem)
					singleRevOp = sOp
				}
				generatedOps = append(generatedOps, singleRevOp)
			}
			reverseOps = generatedOps
		}
	}

	return reverseOps, backupData, err
}
