package ops

import (
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

// NewCreateFile creates a new file creation operation.
// This is a convenience function that creates a FileItem and uses the generic Create() function.
//
// It returns a synthfs.Operation, which can be customized by modifying the returned operation directly.
func NewCreateFile(path string, content []byte, mode fs.FileMode) synthfs.Operation {
	// 1. Create the FsItem representing the file.
	fileItem := synthfs.NewFile(path).
		WithContent(content).
		WithMode(mode)

	// 2. Use the generic Create operation.
	return Create(fileItem)
}
