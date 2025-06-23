package ops

import (
	"io/fs"

	v2 "github.com/arthur-debert/synthfs/pkg/v2/synthfs"
)

// NewCreateFile is a constructor for an operation that creates a file using the v2 API.
// This function demonstrates how specific operation constructors in v2 would leverage
// the generic v2.ops.Create function with a v2.FileItem.
//
// path: The absolute path where the file will be created.
// content: The byte content of the file.
// mode: The file mode (permissions).
//
// It returns a v2.Operation, which can be further configured with WithID, WithDependency, etc.
func NewCreateFile(path string, content []byte, mode fs.FileMode) v2.Operation {
	// 1. Create the FsItem representing the file.
	fileItem := v2.NewFile(path).
		WithContent(content).
		WithMode(mode)

	// 2. Use the generic Create constructor.
	// The generic Create function itself returns a v2.Operation (specifically a *v2.GenericOperation).
	// This operation already has its .Describe() fields populated appropriately by the generic Create.
	// Its ID is also auto-generated based on type and path.
	op := Create(fileItem) // Create is from pkg/v2/synthfs/ops/generic.go

	// The returned 'op' is already a v2.Operation.
	// If we wanted to wrap it further or return a more specific type that embeds
	// v2.GenericOperation but adds more methods, this is where we could do it.
	// For Phase 0, returning the direct result of generic.Create is sufficient
	// to show the pattern.
	return op
}
