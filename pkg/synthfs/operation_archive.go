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

	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

// executeCreateArchive implements archive creation
func (op *SimpleOperation) executeCreateArchive(ctx context.Context, fsys FileSystem) error {
	archiveItem, ok := op.item.(*ArchiveItem)
	if !ok || archiveItem == nil {
		return fmt.Errorf("create_archive operation requires an ArchiveItem")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", archiveItem.Path()).
		Str("format", targets.ArchiveFormat(archiveItem.Format()).String()).
		Int("sources", len(archiveItem.Sources())).
		Msg("executing create archive operation")

	// Phase I, Milestone 4: Verify checksums before execution
	if err := op.verifyChecksums(ctx, fsys); err != nil {
		return fmt.Errorf("archive creation failed checksum verification: %w", err)
	}

	switch archiveItem.Format() {
	case targets.ArchiveFormatTarGz:
		return op.createTarGzArchive(archiveItem, fsys)
	case targets.ArchiveFormatZip:
		return op.createZipArchive(archiveItem, fsys)
	default:
		return fmt.Errorf("unsupported archive format: %s", targets.ArchiveFormat(archiveItem.Format()).String())
	}
}

// executeUnarchive implements archive extraction
func (op *SimpleOperation) executeUnarchive(ctx context.Context, fsys FileSystem) error {
	unarchiveItem, ok := op.item.(*UnarchiveItem)
	if !ok || unarchiveItem == nil {
		return fmt.Errorf("unarchive operation requires an UnarchiveItem")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("archive_path", unarchiveItem.ArchivePath()).
		Str("extract_path", unarchiveItem.ExtractPath()).
		Int("patterns", len(unarchiveItem.Patterns())).
		Msg("executing unarchive operation")

	// Determine archive format from file extension
	format, err := determineArchiveFormat(unarchiveItem.ArchivePath())
	if err != nil {
		return fmt.Errorf("could not determine archive format for %s: %w", unarchiveItem.ArchivePath(), err)
	}

	switch format {
	case ArchiveFormatTarGz:
		return op.extractTarGzArchive(unarchiveItem, fsys)
	case ArchiveFormatZip:
		return op.extractZipArchive(unarchiveItem, fsys)
	default:
		return fmt.Errorf("unsupported archive format: %s", targets.ArchiveFormat(format).String())
	}
}

// validateCreateArchive validates an archive creation operation.
func (op *SimpleOperation) validateCreateArchive(ctx context.Context, fsys FileSystem) error {
	archiveItem, ok := op.item.(*ArchiveItem)
	if !ok || archiveItem == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "create_archive operation requires an ArchiveItem",
		}
	}

	if archiveItem.Path() == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "archive path cannot be empty",
		}
	}

	if len(archiveItem.Sources()) == 0 {
		return &ValidationError{
			Operation: op,
			Reason:    "archive must have at least one source",
		}
	}

	// Check if archive already exists
	if _, err := fs.Stat(fsys, archiveItem.Path()); err == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "archive file already exists",
		}
	}

	// Validate all source paths exist
	for _, source := range archiveItem.Sources() {
		if _, err := fs.Stat(fsys, source); err != nil {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("source path %s does not exist", source),
				Cause:     err,
			}
		}
	}

	return nil
}

// validateUnarchive validates an unarchive operation.
func (op *SimpleOperation) validateUnarchive(ctx context.Context, fsys FileSystem) error {
	// First check if we have the right item type
	if op.item == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "unarchive operation requires an UnarchiveItem",
		}
	}
	
	unarchiveItem, ok := op.item.(*UnarchiveItem)
	if !ok {
		return &ValidationError{
			Operation: op,
			Reason:    "expected UnarchiveItem but got different type",
		}
	}

	// Validate archive path is not empty
	if unarchiveItem.ArchivePath() == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "archive path cannot be empty",
		}
	}

	// Validate extract path is not empty
	if unarchiveItem.ExtractPath() == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "extract path cannot be empty",
		}
	}

	// Validate archive format
	if _, err := determineArchiveFormat(unarchiveItem.ArchivePath()); err != nil {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("unsupported archive format for file: %s", unarchiveItem.ArchivePath()),
			Cause:     err,
		}
	}

	// Check if archive exists
	if _, err := fs.Stat(fsys, unarchiveItem.ArchivePath()); err != nil {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("archive file %s does not exist", unarchiveItem.ArchivePath()),
			Cause:     err,
		}
	}

	// Check if extract path exists
	if stat, err := fs.Stat(fsys, unarchiveItem.ExtractPath()); err != nil {
		// Extract path doesn't exist - will be created during extraction
		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("extract_path", unarchiveItem.ExtractPath()).
			Msg("extract path does not exist yet")
	} else if !stat.IsDir() {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("extract path %s exists but is not a directory", unarchiveItem.ExtractPath()),
		}
	}

	return nil
}

// rollbackUnarchive rolls back an unarchive operation by removing extracted files.
func (op *SimpleOperation) rollbackUnarchive(ctx context.Context, fsys FileSystem) error {
	unarchiveItem, ok := op.item.(*UnarchiveItem)
	if !ok || unarchiveItem == nil {
		return fmt.Errorf("cannot rollback: no UnarchiveItem found")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("extract_path", unarchiveItem.ExtractPath()).
		Msg("rolling back unarchive operation")

	// Remove the entire extraction directory
	if err := fsys.RemoveAll(unarchiveItem.ExtractPath()); err != nil {
		Logger().Warn().
			Str("op_id", string(op.ID())).
			Str("extract_path", unarchiveItem.ExtractPath()).
			Err(err).
			Msg("rollback remove failed (may be acceptable)")
	}

	return nil
}

// reverseCreateArchive generates operations to reverse an archive creation.
func (op *SimpleOperation) reverseCreateArchive(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	path := op.description.Path

	// Create a delete operation to remove the archive
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"delete",
		path,
	)

	return []Operation{reverseOp}, nil, nil
}

// reverseUnarchive generates operations to reverse an unarchive operation.
func (op *SimpleOperation) reverseUnarchive(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	unarchiveItem, ok := op.item.(*UnarchiveItem)
	if !ok || unarchiveItem == nil {
		return nil, nil, fmt.Errorf("cannot reverse: no UnarchiveItem found")
	}

	// Create a delete operation to remove the extracted files
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"delete",
		unarchiveItem.ExtractPath(),
	)

	return []Operation{reverseOp}, nil, nil
}

// Helper methods for archive creation

// createTarGzArchive creates a tar.gz archive
func (op *SimpleOperation) createTarGzArchive(archiveItem *ArchiveItem, fsys FileSystem) error {
	// Create a buffer to hold the archive data
	var buf bytes.Buffer

	// Create gzip writer
	gzipWriter := gzip.NewWriter(&buf)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			Logger().Warn().Err(err).Msg("failed to close gzip writer")
		}
	}()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if err := tarWriter.Close(); err != nil {
			Logger().Warn().Err(err).Msg("failed to close tar writer")
		}
	}()

	// Add each source to the archive
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

	// Create parent directory if needed
	dir := filepath.Dir(archiveItem.Path())
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Write the complete archive to the filesystem
	return fsys.WriteFile(archiveItem.Path(), buf.Bytes(), 0644)
}

// createZipArchive creates a zip archive
func (op *SimpleOperation) createZipArchive(archiveItem *ArchiveItem, fsys FileSystem) error {
	// Create a buffer to hold the archive data
	var buf bytes.Buffer

	// Create zip writer
	zipWriter := zip.NewWriter(&buf)
	defer func() {
		if err := zipWriter.Close(); err != nil {
			Logger().Warn().Err(err).Msg("failed to close zip writer")
		}
	}()

	// Add each source to the archive
	for _, source := range archiveItem.Sources() {
		if err := op.addToZipArchive(zipWriter, source, fsys); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", source, err)
		}
	}

	// Close writer to flush data
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %w", err)
	}

	// Create parent directory if needed
	dir := filepath.Dir(archiveItem.Path())
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
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
			if err := file.Close(); err != nil {
				Logger().Warn().Err(err).Str("path", sourcePath).Msg("failed to close file")
			}
		}()

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to write file content for %s: %w", sourcePath, err)
		}
	}

	// If it's a directory, recursively add its contents
	if info.IsDir() {
		entries, err := fs.ReadDir(fsys, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", sourcePath, err)
		}

		for _, entry := range entries {
			childPath := filepath.Join(sourcePath, entry.Name())
			if err := op.addToTarArchive(tarWriter, childPath, fsys); err != nil {
				return err
			}
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

	// If it's a directory, create directory entry and recurse
	if info.IsDir() {
		if !strings.HasSuffix(archivePath, "/") {
			archivePath += "/"
		}
		_, err := zipWriter.Create(archivePath)
		if err != nil {
			return fmt.Errorf("failed to create directory entry: %w", err)
		}

		// Recursively add directory contents
		entries, err := fs.ReadDir(fsys, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", sourcePath, err)
		}

		for _, entry := range entries {
			childPath := filepath.Join(sourcePath, entry.Name())
			if err := op.addToZipArchive(zipWriter, childPath, fsys); err != nil {
				return err
			}
		}
		return nil
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
		if err := file.Close(); err != nil {
			Logger().Warn().Err(err).Str("path", sourcePath).Msg("failed to close file")
		}
	}()

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("failed to write file content for %s: %w", sourcePath, err)
	}

	return nil
}

// Helper methods for archive extraction

// extractTarGzArchive extracts a tar.gz archive
func (op *SimpleOperation) extractTarGzArchive(unarchiveItem *UnarchiveItem, fsys FileSystem) error {
	// Open archive file
	file, err := fsys.Open(unarchiveItem.ArchivePath())
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", unarchiveItem.ArchivePath(), err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			Logger().Warn().Err(err).Str("path", unarchiveItem.ArchivePath()).Msg("failed to close archive file")
		}
	}()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			Logger().Warn().Err(err).Msg("failed to close gzip reader")
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
			Logger().Warn().
				Str("op_id", string(op.ID())).
				Str("path", header.Name).
				Str("type", string(header.Typeflag)).
				Msg("skipping unsupported file type")
			continue

		default:
			Logger().Warn().
				Str("op_id", string(op.ID())).
				Str("path", header.Name).
				Str("type", string(header.Typeflag)).
				Msg("skipping unknown file type")
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
		if err := file.Close(); err != nil {
			Logger().Warn().Err(err).Str("path", unarchiveItem.ArchivePath()).Msg("failed to close archive file")
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
	_, err = io.ReadFull(file, content)
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
		if err := reader.Close(); err != nil {
			Logger().Warn().Err(err).Msg("failed to close zip file reader")
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