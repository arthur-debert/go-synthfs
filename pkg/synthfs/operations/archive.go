package operations

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// CreateArchiveOperation represents an archive creation operation.
type CreateArchiveOperation struct {
	*BaseOperation
}

// NewCreateArchiveOperation creates a new archive creation operation.
func NewCreateArchiveOperation(id core.OperationID, archivePath string) *CreateArchiveOperation {
	return &CreateArchiveOperation{
		BaseOperation: NewBaseOperation(id, "create_archive", archivePath),
	}
}

// Execute creates the archive.
func (op *CreateArchiveOperation) Execute(ctx context.Context, fsys interface{}) error {
	// Get sources - first try from item, then from details
	var sources []string
	var format interface{}

	if item := op.GetItem(); item != nil {
		if archiveItem, ok := item.(interface {
			Sources() []string
			Format() string
		}); ok {
			sources = archiveItem.Sources()
			format = archiveItem.Format()
		}
	}

	// Fallback to details
	if len(sources) == 0 {
		if detailSources, ok := op.description.Details["sources"].([]string); ok {
			sources = detailSources
		}
	}
	if len(sources) == 0 {
		return fmt.Errorf("create_archive operation requires sources")
	}

	if format == nil {
		format = op.description.Details["format"]
	}
	if format == nil {
		return fmt.Errorf("create_archive operation requires format")
	}

	// For now, we'll need to use the OS filesystem for archive creation
	// This is a limitation we'll address in future iterations
	archivePath := op.description.Path

	// Determine archive type based on format or file extension
	formatStr := fmt.Sprintf("%v", format)
	switch strings.ToLower(formatStr) {
	case "zip":
		return op.createZipArchive(archivePath, sources, fsys)
	case "tar", "tar.gz", "tgz":
		return op.createTarArchive(archivePath, sources, fsys, strings.HasSuffix(strings.ToLower(archivePath), ".gz"))
	default:
		// Try to determine from file extension
		ext := strings.ToLower(filepath.Ext(archivePath))
		switch ext {
		case ".zip":
			return op.createZipArchive(archivePath, sources, fsys)
		case ".tar":
			return op.createTarArchive(archivePath, sources, fsys, false)
		case ".gz", ".tgz":
			return op.createTarArchive(archivePath, sources, fsys, true)
		default:
			return fmt.Errorf("unsupported archive format: %s", formatStr)
		}
	}
}

// createZipArchive creates a ZIP archive.
func (op *CreateArchiveOperation) createZipArchive(archivePath string, sources []string, fsys interface{}) error {
	// This is a simplified implementation
	// In a real implementation, we'd use the filesystem interface
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer func() { _ = file.Close() }()

	zipWriter := zip.NewWriter(file)
	defer func() { _ = zipWriter.Close() }()

	// Add sources to archive
	for _, source := range sources {
		// TODO: Implement actual file reading through fsys interface
		// For now, this is a placeholder
		_, err := zipWriter.Create(filepath.Base(source))
		if err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", source, err)
		}
	}

	return nil
}

// createTarArchive creates a TAR or TAR.GZ archive.
func (op *CreateArchiveOperation) createTarArchive(archivePath string, sources []string, fsys interface{}, compress bool) error {
	// This is a simplified implementation
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var tarWriter *tar.Writer
	if compress {
		gzWriter := gzip.NewWriter(file)
		defer func() { _ = gzWriter.Close() }()
		tarWriter = tar.NewWriter(gzWriter)
	} else {
		tarWriter = tar.NewWriter(file)
	}
	defer func() { _ = tarWriter.Close() }()

	// Add sources to archive
	for _, source := range sources {
		// TODO: Implement actual file reading through fsys interface
		// For now, this is a placeholder
		header := &tar.Header{
			Name: filepath.Base(source),
			Mode: 0644,
			Size: 0, // Would be actual file size
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write header for %s: %w", source, err)
		}
	}

	return nil
}

// ExecuteV2 performs the archive creation with execution context support.
func (op *CreateArchiveOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the archive can be created.
func (op *CreateArchiveOperation) Validate(ctx context.Context, fsys interface{}) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	// Check sources from item first
	var sources []string
	if item := op.GetItem(); item != nil {
		if archiveItem, ok := item.(interface{ Sources() []string }); ok {
			sources = archiveItem.Sources()
		}
	}

	// If no sources from item, check description details
	if len(sources) == 0 {
		if detailSources, ok := op.description.Details["sources"].([]string); ok {
			sources = detailSources
		}
	}

	if len(sources) == 0 {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "must specify at least one source",
		}
	}

	// Check if sources exist
	if stat, ok := getStatMethod(fsys); ok {
		for _, source := range sources {
			if _, err := stat(source); err != nil {
				return &core.ValidationError{
					OperationID:   op.ID(),
					OperationDesc: op.Describe(),
					Reason:        fmt.Sprintf("source does not exist: %s", source),
					Cause:         err,
				}
			}
		}
	}

	return nil
}

// Rollback removes the created archive.
func (op *CreateArchiveOperation) Rollback(ctx context.Context, fsys interface{}) error {
	remove, ok := getRemoveMethod(fsys)
	if !ok {
		return fmt.Errorf("filesystem does not support Remove")
	}

	// Remove the archive
	_ = remove(op.description.Path) // Ignore error - might not exist
	return nil
}

// UnarchiveOperation represents an archive extraction operation.
type UnarchiveOperation struct {
	*BaseOperation
}

// NewUnarchiveOperation creates a new unarchive operation.
func NewUnarchiveOperation(id core.OperationID, archivePath string) *UnarchiveOperation {
	return &UnarchiveOperation{
		BaseOperation: NewBaseOperation(id, "unarchive", archivePath),
	}
}

// Execute extracts the archive.
func (op *UnarchiveOperation) Execute(ctx context.Context, fsys interface{}) error {
	// Get extract path from details
	extractPath, ok := op.description.Details["extract_path"].(string)
	if !ok || extractPath == "" {
		extractPath = "." // Default to current directory
	}

	// Get patterns from details (optional)
	patterns, _ := op.description.Details["patterns"].([]string)

	archivePath := op.description.Path

	// Determine archive type based on file extension
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".zip":
		return op.extractZipArchive(archivePath, extractPath, patterns, fsys)
	case ".tar":
		return op.extractTarArchive(archivePath, extractPath, patterns, fsys, false)
	case ".gz", ".tgz":
		if strings.HasSuffix(archivePath, ".tar.gz") || ext == ".tgz" {
			return op.extractTarArchive(archivePath, extractPath, patterns, fsys, true)
		}
		return fmt.Errorf("unsupported archive format: %s", ext)
	default:
		return fmt.Errorf("unsupported archive format: %s", ext)
	}
}

// extractZipArchive extracts a ZIP archive.
func (op *UnarchiveOperation) extractZipArchive(archivePath, extractPath string, patterns []string, fsys interface{}) error {
	// This is a simplified implementation
	// In a real implementation, we'd use the filesystem interface throughout

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Get filesystem methods
	mkdirAll, _ := getMkdirAllMethod(fsys)
	writeFile, _ := getWriteFileMethod(fsys)

	for _, file := range reader.File {
		// Check patterns if provided
		if len(patterns) > 0 && !matchesPatterns(file.Name, patterns) {
			continue
		}

		path := filepath.Join(extractPath, file.Name)

		if file.FileInfo().IsDir() {
			if mkdirAll != nil {
				_ = mkdirAll(path, file.Mode())
			}
			continue
		}

		// Create directory for file
		if mkdirAll != nil {
			_ = mkdirAll(filepath.Dir(path), 0755)
		}

		// Extract file
		if writeFile != nil {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			_ = writeFile(path, content, file.Mode())
		}
	}

	return nil
}

// extractTarArchive extracts a TAR or TAR.GZ archive.
func (op *UnarchiveOperation) extractTarArchive(archivePath, extractPath string, patterns []string, fsys interface{}, compressed bool) error {
	// This is a simplified implementation
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	var tarReader *tar.Reader
	if compressed {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() { _ = gzReader.Close() }()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	// Get filesystem methods
	mkdirAll, _ := getMkdirAllMethod(fsys)
	writeFile, _ := getWriteFileMethod(fsys)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Check patterns if provided
		if len(patterns) > 0 && !matchesPatterns(header.Name, patterns) {
			continue
		}

		path := filepath.Join(extractPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if mkdirAll != nil {
				_ = mkdirAll(path, os.FileMode(header.Mode))
			}
		case tar.TypeReg:
			// Create directory for file
			if mkdirAll != nil {
				_ = mkdirAll(filepath.Dir(path), 0755)
			}

			// Extract file
			if writeFile != nil {
				content, _ := io.ReadAll(tarReader)
				_ = writeFile(path, content, os.FileMode(header.Mode))
			}
		}
	}

	return nil
}

// matchesPatterns checks if a name matches any of the provided patterns.
func matchesPatterns(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
		// Also check if the name contains the pattern as a substring
		if strings.Contains(name, pattern) {
			return true
		}
	}

	return false
}

// ExecuteV2 performs the unarchive with execution context support.
func (op *UnarchiveOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the unarchive operation can be performed.
func (op *UnarchiveOperation) Validate(ctx context.Context, fsys interface{}) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	// Check if archive exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(op.description.Path); err != nil {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "archive does not exist",
				Cause:         err,
			}
		}
	}

	return nil
}

// Rollback for unarchive would need to track and remove all extracted files.
func (op *UnarchiveOperation) Rollback(ctx context.Context, fsys interface{}) error {
	// TODO: Implement tracking of extracted files for proper rollback
	return fmt.Errorf("rollback of unarchive operations not yet implemented")
}
