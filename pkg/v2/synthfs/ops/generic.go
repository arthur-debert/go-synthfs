package ops

import (
	v2 "github.com/arthur-debert/synthfs/pkg/v2/synthfs" // Alias to avoid conflict if this package was also named synthfs
)

// Create constructs a generic operation to create the given FsItem.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Create(item v2.FsItem) v2.Operation {
	opID := v2.OperationID("create_" + item.Type() + "_" + item.Path()) // Basic ID generation
	baseOp := v2.NewBaseOperation(opID, "create_"+item.Type(), item.Path())

	// Set item-specific details for description
	switch specificItem := item.(type) {
	case *v2.FileItem:
		baseOp.SetDescriptionDetail("content_length", len(specificItem.Content()))
		baseOp.SetDescriptionDetail("mode", specificItem.Mode().String())
	case *v2.DirectoryItem:
		baseOp.SetDescriptionDetail("mode", specificItem.Mode().String())
	case *v2.SymlinkItem:
		baseOp.SetDescriptionDetail("target", specificItem.Target())
	case *v2.ArchiveItem:
		baseOp.SetDescriptionDetail("format", specificItem.Format().String())
		baseOp.SetDescriptionDetail("source_count", len(specificItem.Sources()))
	}

	return &v2.GenericOperation{
		BaseOperation: baseOp,
		Item:          item,
	}
}

// Delete constructs a generic operation to delete the item at the given path.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Delete(path string) v2.Operation {
	opID := v2.OperationID("delete_" + path) // Basic ID generation
	baseOp := v2.NewBaseOperation(opID, "delete", path)
	// In a real implementation, 'delete' might inspect the path to determine type,
	// or be a more generic "remove" operation.
	// For description, we might want to add what type is expected to be deleted if known.
	return &v2.GenericOperation{
		BaseOperation: baseOp,
	}
}

// Copy constructs a generic operation to copy an item from src to dst.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Copy(src, dst string) v2.Operation {
	opID := v2.OperationID("copy_" + src + "_to_" + dst) // Basic ID generation
	baseOp := v2.NewBaseOperation(opID, "copy", src)     // Primary path is src for description
	baseOp.SetDescriptionDetail("destination", dst)
	return &v2.GenericOperation{
		BaseOperation: baseOp,
		SrcPath:       src,
		DstPath:       dst,
	}
}

// Move constructs a generic operation to move an item from src to dst.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Move(src, dst string) v2.Operation {
	opID := v2.OperationID("move_" + src + "_to_" + dst) // Basic ID generation
	baseOp := v2.NewBaseOperation(opID, "move", src)     // Primary path is src for description
	baseOp.SetDescriptionDetail("destination", dst)
	return &v2.GenericOperation{
		BaseOperation: baseOp,
		SrcPath:       src,
		DstPath:       dst,
	}
}
