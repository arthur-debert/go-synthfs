package ops

import "github.com/arthur-debert/synthfs/pkg/synthfs"

// Create constructs a generic operation to create the given FsItem.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Create(item synthfs.FsItem) synthfs.Operation {
	opID := synthfs.OperationID("create_" + item.Type() + "_" + item.Path()) // Basic ID generation
	baseOp := synthfs.NewBaseOperation(opID, "create_"+item.Type(), item.Path())

	// Set item-specific details for description
	switch specificItem := item.(type) {
	case *synthfs.FileItem:
		baseOp.SetDescriptionDetail("content_length", len(specificItem.Content()))
		baseOp.SetDescriptionDetail("mode", specificItem.Mode().String())
	case *synthfs.DirectoryItem:
		baseOp.SetDescriptionDetail("mode", specificItem.Mode().String())
	case *synthfs.SymlinkItem:
		baseOp.SetDescriptionDetail("target", specificItem.Target())
	case *synthfs.ArchiveItem:
		baseOp.SetDescriptionDetail("format", specificItem.Format().String())
		baseOp.SetDescriptionDetail("source_count", len(specificItem.Sources()))
	}

	return &synthfs.GenericOperation{
		BaseOperation: baseOp,
		Item:          item,
	}
}

// Delete constructs a generic operation to delete the item at the given path.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Delete(path string) synthfs.Operation {
	opID := synthfs.OperationID("delete_" + path) // Basic ID generation
	baseOp := synthfs.NewBaseOperation(opID, "delete", path)
	// In a real implementation, 'delete' might inspect the path to determine type,
	// or be a more generic "remove" operation.
	// For description, we might want to add what type is expected to be deleted if known.
	return &synthfs.GenericOperation{
		BaseOperation: baseOp,
	}
}

// Copy constructs a generic operation to copy an item from src to dst.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Copy(src, dst string) synthfs.Operation {
	opID := synthfs.OperationID("copy_" + src + "_to_" + dst) // Basic ID generation
	baseOp := synthfs.NewBaseOperation(opID, "copy", src)     // Primary path is src for description
	baseOp.SetDescriptionDetail("destination", dst)
	return &synthfs.GenericOperation{
		BaseOperation: baseOp,
		SrcPath:       src,
		DstPath:       dst,
	}
}

// Move constructs a generic operation to move an item from src to dst.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Move(src, dst string) synthfs.Operation {
	opID := synthfs.OperationID("move_" + src + "_to_" + dst) // Basic ID generation
	baseOp := synthfs.NewBaseOperation(opID, "move", src)     // Primary path is src for description
	baseOp.SetDescriptionDetail("destination", dst)
	return &synthfs.GenericOperation{
		BaseOperation: baseOp,
		SrcPath:       src,
		DstPath:       dst,
	}
}
