package ops

import (
	"io/fs"

	v2 "github.com/arthur-debert/synthfs/pkg/v2/synthfs"
)

// NewCreateDirectory is a constructor for an operation that creates a directory using the v2 API.
// This function demonstrates how specific operation constructors in v2 would leverage
// the generic v2.ops.Create function with a v2.DirectoryItem.
//
// path: The absolute path where the directory will be created.
// mode: The directory mode (permissions).
//
// It returns a v2.Operation, which can be further configured with WithID, WithDependency, etc.
func NewCreateDirectory(path string, mode fs.FileMode) v2.Operation {
	// 1. Create the FsItem representing the directory.
	dirItem := v2.NewDirectory(path).
		WithMode(mode)

	// 2. Use the generic Create constructor.
	op := Create(dirItem) // Create is from pkg/v2/synthfs/ops/generic.go

	// Similar to NewCreateFile, for Phase 0, returning the direct result
	// of generic.Create is sufficient to show the pattern.
	return op
}
