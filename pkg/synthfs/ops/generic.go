package ops

import "github.com/arthur-debert/synthfs/pkg/synthfs"

// Create constructs a generic operation to create the given FsItem.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Create(item synthfs.FsItem) synthfs.Operation {
	opID := synthfs.OperationID("create_" + item.Type() + "_" + item.Path()) // Basic ID generation
	op := synthfs.NewSimpleOperation(opID, "create_"+item.Type(), item.Path())

	// Set the item for this create operation
	op.SetItem(item)

	// Set item-specific details for description
	switch specificItem := item.(type) {
	case *synthfs.FileItem:
		op.SetDescriptionDetail("content_length", len(specificItem.Content()))
		op.SetDescriptionDetail("mode", specificItem.Mode().String())
	case *synthfs.DirectoryItem:
		op.SetDescriptionDetail("mode", specificItem.Mode().String())
	case *synthfs.SymlinkItem:
		op.SetDescriptionDetail("target", specificItem.Target())
	case *synthfs.ArchiveItem:
		op.SetDescriptionDetail("format", specificItem.Format().String())
		op.SetDescriptionDetail("source_count", len(specificItem.Sources()))
	}

	return op
}

// Delete constructs a generic operation to delete the item at the given path.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Delete(path string) synthfs.Operation {
	opID := synthfs.OperationID("delete_" + path) // Basic ID generation
	op := synthfs.NewSimpleOperation(opID, "delete", path)
	// In a real implementation, 'delete' might inspect the path to determine type,
	// or be a more generic "remove" operation.
	// For description, we might want to add what type is expected to be deleted if known.
	return op
}

// Copy constructs a generic operation to copy an item from src to dst.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Copy(src, dst string) synthfs.Operation {
	opID := synthfs.OperationID("copy_" + src + "_to_" + dst) // Basic ID generation
	op := synthfs.NewSimpleOperation(opID, "copy", src)       // Primary path is src for description
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)
	return op
}

// Move constructs a generic operation to move an item from src to dst.
// For Phase 0, this returns a GenericOperation with stubbed methods.
func Move(src, dst string) synthfs.Operation {
	opID := synthfs.OperationID("move_" + src + "_to_" + dst) // Basic ID generation
	op := synthfs.NewSimpleOperation(opID, "move", src)       // Primary path is src for description
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)
	return op
}
