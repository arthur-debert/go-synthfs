package synthfs

import (
	"crypto/md5"
	"fmt"
	"io"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Filesystem interfaces are now in the filesystem package
// Type aliases are provided in types.go for backward compatibility

// OSFileSystem is now a type alias for the filesystem package version
type OSFileSystem = filesystem.OSFileSystem

// NewOSFileSystem creates a new OS-based filesystem rooted at the given path
func NewOSFileSystem(root string) *OSFileSystem {
	return filesystem.NewOSFileSystem(root)
}

// ComputeFileChecksum calculates the MD5 checksum and gathers file metadata.
func ComputeFileChecksum(fsys FullFileSystem, filePath string) (*ChecksumRecord, error) {
	info, err := fsys.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if info.IsDir() {
		return nil, nil
	}

	file, err := fsys.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s for checksumming: %w", filePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			Logger().Warn().Err(closeErr).Str("file", filePath).Msg("failed to close file after checksumming")
		}
	}()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to calculate checksum for %s: %w", filePath, err)
	}

	checksum := &ChecksumRecord{
		Path:         filePath,
		MD5:          fmt.Sprintf("%x", hash.Sum(nil)),
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		ChecksumTime: time.Now(),
	}

	return checksum, nil
}

// ReadOnlyWrapper is now a type alias for the filesystem package version
type ReadOnlyWrapper = filesystem.ReadOnlyWrapper

// NewReadOnlyWrapper creates a new wrapper for an fs.FS
func NewReadOnlyWrapper(fsys ReadFS) *ReadOnlyWrapper {
	return filesystem.NewReadOnlyWrapper(fsys)
}