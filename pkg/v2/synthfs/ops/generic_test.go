package ops_test

import (
	"context"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	v2 "github.com/arthur-debert/synthfs/pkg/v2/synthfs"
	v2ops "github.com/arthur-debert/synthfs/pkg/v2/synthfs/ops"
)

func TestCreateOperation_File(t *testing.T) {
	filePath := "/tmp/file.txt"
	fileContent := []byte("content")
	fileMode := fs.FileMode(0666)
	fileItem := v2.NewFile(filePath).WithContent(fileContent).WithMode(fileMode)

	op := v2ops.Create(fileItem)

	if op == nil {
		t.Fatal("Create(fileItem) returned nil")
	}

	desc := op.Describe()
	if desc.Type != "create_file" {
		t.Errorf("Expected desc.Type 'create_file', got '%s'", desc.Type)
	}
	if desc.Path != filePath {
		t.Errorf("Expected desc.Path '%s', got '%s'", filePath, desc.Path)
	}
	if val, ok := desc.Details["content_length"]; !ok || val.(int) != len(fileContent) {
		t.Errorf("Expected detail content_length %d, got %v", len(fileContent), val)
	}
	if val, ok := desc.Details["mode"]; !ok || val.(string) != fileMode.String() {
		t.Errorf("Expected detail mode %s, got %v", fileMode.String(), val)
	}

	expectedIDPrefix := "create_file_" + filePath
	if !strings.HasPrefix(string(op.ID()), expectedIDPrefix) {
		t.Errorf("Expected ID to start with '%s', got '%s'", expectedIDPrefix, op.ID())
	}

	// Test chainable methods
	opID := v2.OperationID("test-id")
	depID := v2.OperationID("dep-id")
	chainedOp := op.WithID(opID).WithDependency(depID)

	if chainedOp.ID() != opID {
		t.Errorf("WithID failed: expected '%s', got '%s'", opID, chainedOp.ID())
	}
	if len(chainedOp.Dependencies()) != 1 || chainedOp.Dependencies()[0] != depID {
		t.Errorf("WithDependency failed: expected ['%s'], got %v", depID, chainedOp.Dependencies())
	}

	// Test stub methods (should not panic)
	ctx := context.Background()
	var mockFs v2.FileSystem // Can be nil for these stubs
	if err := op.Execute(ctx, mockFs); err != nil {
		t.Errorf("Execute() stub failed: %v", err)
	}
	if err := op.Validate(ctx, mockFs); err != nil {
		t.Errorf("Validate() stub failed: %v", err)
	}
	if err := op.Rollback(ctx, mockFs); err != nil {
		t.Errorf("Rollback() stub failed: %v", err)
	}
}

func TestCreateOperation_Directory(t *testing.T) {
	dirPath := "/tmp/dir"
	dirMode := fs.FileMode(0777)
	dirItem := v2.NewDirectory(dirPath).WithMode(dirMode)

	op := v2ops.Create(dirItem)
	if op == nil {
		t.Fatal("Create(dirItem) returned nil")
	}

	desc := op.Describe()
	if desc.Type != "create_directory" {
		t.Errorf("Expected desc.Type 'create_directory', got '%s'", desc.Type)
	}
	if desc.Path != dirPath {
		t.Errorf("Expected desc.Path '%s', got '%s'", dirPath, desc.Path)
	}
	if val, ok := desc.Details["mode"]; !ok || val.(string) != dirMode.String() {
		t.Errorf("Expected detail mode %s, got %v", dirMode.String(), val)
	}
	expectedIDPrefix := "create_directory_" + dirPath
	if !strings.HasPrefix(string(op.ID()), expectedIDPrefix) {
		t.Errorf("Expected ID to start with '%s', got '%s'", expectedIDPrefix, op.ID())
	}
}

func TestCreateOperation_Symlink(t *testing.T) {
	linkPath := "/tmp/link"
	targetPath := "/tmp/target"
	symlinkItem := v2.NewSymlink(linkPath, targetPath)

	op := v2ops.Create(symlinkItem)
	if op == nil {
		t.Fatal("Create(symlinkItem) returned nil")
	}

	desc := op.Describe()
	if desc.Type != "create_symlink" {
		t.Errorf("Expected desc.Type 'create_symlink', got '%s'", desc.Type)
	}
	if desc.Path != linkPath {
		t.Errorf("Expected desc.Path '%s', got '%s'", linkPath, desc.Path)
	}
	if val, ok := desc.Details["target"]; !ok || val.(string) != targetPath {
		t.Errorf("Expected detail target '%s', got '%v'", targetPath, val)
	}
}

func TestCreateOperation_Archive(t *testing.T) {
	archivePath := "/tmp/archive.zip"
	sources := []string{"/a", "/b"}
	format := v2.ArchiveFormatZip
	archiveItem := v2.NewArchive(archivePath, format, sources)

	op := v2ops.Create(archiveItem)
	if op == nil {
		t.Fatal("Create(archiveItem) returned nil")
	}

	desc := op.Describe()
	if desc.Type != "create_archive" {
		t.Errorf("Expected desc.Type 'create_archive', got '%s'", desc.Type)
	}
	if desc.Path != archivePath {
		t.Errorf("Expected desc.Path '%s', got '%s'", archivePath, desc.Path)
	}
	if val, ok := desc.Details["format"]; !ok || val.(string) != format.String() {
		t.Errorf("Expected detail format '%s', got '%v'", format.String(), val)
	}
	if val, ok := desc.Details["source_count"]; !ok || val.(int) != len(sources) {
		t.Errorf("Expected detail source_count %d, got %v", len(sources), val)
	}
}

func TestDeleteOperation(t *testing.T) {
	deletePath := "/tmp/item_to_delete"
	op := v2ops.Delete(deletePath)
	if op == nil {
		t.Fatal("Delete() returned nil")
	}

	desc := op.Describe()
	if desc.Type != "delete" {
		t.Errorf("Expected desc.Type 'delete', got '%s'", desc.Type)
	}
	if desc.Path != deletePath {
		t.Errorf("Expected desc.Path '%s', got '%s'", deletePath, desc.Path)
	}
	expectedID := "delete_" + deletePath
	if op.ID() != v2.OperationID(expectedID) {
		t.Errorf("Expected ID '%s', got '%s'", expectedID, op.ID())
	}
}

func TestCopyOperation(t *testing.T) {
	srcPath := "/tmp/source_item"
	dstPath := "/tmp/dest_item"
	op := v2ops.Copy(srcPath, dstPath)
	if op == nil {
		t.Fatal("Copy() returned nil")
	}

	desc := op.Describe()
	if desc.Type != "copy" {
		t.Errorf("Expected desc.Type 'copy', got '%s'", desc.Type)
	}
	if desc.Path != srcPath { // Primary path is src
		t.Errorf("Expected desc.Path '%s', got '%s'", srcPath, desc.Path)
	}
	if val, ok := desc.Details["destination"]; !ok || val.(string) != dstPath {
		t.Errorf("Expected detail destination '%s', got '%v'", dstPath, val)
	}
	expectedID := "copy_" + srcPath + "_to_" + dstPath
	if op.ID() != v2.OperationID(expectedID) {
		t.Errorf("Expected ID '%s', got '%s'", expectedID, op.ID())
	}
}

func TestMoveOperation(t *testing.T) {
	srcPath := "/tmp/source_move_item"
	dstPath := "/tmp/dest_move_item"
	op := v2ops.Move(srcPath, dstPath)
	if op == nil {
		t.Fatal("Move() returned nil")
	}

	desc := op.Describe()
	if desc.Type != "move" {
		t.Errorf("Expected desc.Type 'move', got '%s'", desc.Type)
	}
	if desc.Path != srcPath { // Primary path is src
		t.Errorf("Expected desc.Path '%s', got '%s'", srcPath, desc.Path)
	}
	if val, ok := desc.Details["destination"]; !ok || val.(string) != dstPath {
		t.Errorf("Expected detail destination '%s', got '%v'", dstPath, val)
	}
	expectedID := "move_" + srcPath + "_to_" + dstPath
	if op.ID() != v2.OperationID(expectedID) {
		t.Errorf("Expected ID '%s', got '%s'", expectedID, op.ID())
	}
}

// TestSpecificConstructors tests the NewCreateFile and NewCreateDirectory
// from pkg/v2/synthfs/ops (which should be in the same package `ops_test` can see)
// but are defined in create_file.go and create_dir.go.
// These tests ensure they correctly use the generic v2ops.Create.

func TestNewCreateFileViaOps(t *testing.T) {
	filePath := "/tmp/specific_file.txt"
	fileContent := []byte("specific content")
	fileMode := fs.FileMode(0654)

	op := v2ops.NewCreateFile(filePath, fileContent, fileMode)
	if op == nil {
		t.Fatal("v2ops.NewCreateFile returned nil")
	}

	desc := op.Describe()
	if desc.Type != "create_file" {
		t.Errorf("Expected desc.Type 'create_file', got '%s'", desc.Type)
	}
	if desc.Path != filePath {
		t.Errorf("Expected desc.Path '%s', got '%s'", filePath, desc.Path)
	}

	// Verify that it's a GenericOperation by trying to access item (not directly possible without type assertion)
	// For now, checking description details is a good proxy.
	if val, ok := desc.Details["content_length"]; !ok || val.(int) != len(fileContent) {
		t.Errorf("Expected detail content_length %d, got %v", len(fileContent), val)
	}
	if val, ok := desc.Details["mode"]; !ok || val.(string) != fileMode.String() {
		t.Errorf("Expected detail mode %s, got %v", fileMode.String(), val)
	}

	// Check if the item within GenericOperation was set correctly.
	// Access internal item field (NOTE: this is white-box testing, generally not ideal,
	// but useful here to confirm the internal delegation for Phase 0)
	// Need to export 'Item' field from GenericOperation or add an accessor for proper testing.
	// For now, we rely on the Describe details.
	// To make this testable, we'd need to expose `item` or test behavior that depends on `item`.
	// For Phase 0, the description check is the main verification of delegation.

	item := op.GetItem()
	if item == nil {
		t.Fatal("op.GetItem() returned nil for NewCreateFile operation")
	}
	fileItem, ok := item.(*v2.FileItem)
	if !ok {
		t.Fatalf("Item within operation is not *v2.FileItem, got %T", item)
	}
	if !reflect.DeepEqual(fileItem.Content(), fileContent) {
		t.Errorf("FileItem content mismatch: expected %s, got %s", string(fileContent), string(fileItem.Content()))
	}
	if fileItem.Mode() != fileMode {
		t.Errorf("FileItem mode mismatch: expected %v, got %v", fileMode, fileItem.Mode())
	}
	if fileItem.Path() != filePath {
		t.Errorf("FileItem path mismatch: expected %s, got %s", filePath, fileItem.Path())
	}
}

func TestNewCreateDirectoryViaOps(t *testing.T) {
	dirPath := "/tmp/specific_dir"
	dirMode := fs.FileMode(0765)

	op := v2ops.NewCreateDirectory(dirPath, dirMode)
	if op == nil {
		t.Fatal("v2ops.NewCreateDirectory returned nil")
	}

	desc := op.Describe()
	if desc.Type != "create_directory" {
		t.Errorf("Expected desc.Type 'create_directory', got '%s'", desc.Type)
	}
	if desc.Path != dirPath {
		t.Errorf("Expected desc.Path '%s', got '%s'", dirPath, desc.Path)
	}
	if val, ok := desc.Details["mode"]; !ok || val.(string) != dirMode.String() {
		t.Errorf("Expected detail mode %s, got %v", dirMode.String(), val)
	}

	item := op.GetItem()
	if item == nil {
		t.Fatal("op.GetItem() returned nil for NewCreateDirectory operation")
	}
	dirItem, ok := item.(*v2.DirectoryItem)
	if !ok {
		t.Fatalf("Item within operation is not *v2.DirectoryItem, got %T", item)
	}
	if dirItem.Mode() != dirMode {
		t.Errorf("DirectoryItem mode mismatch: expected %v, got %v", dirMode, dirItem.Mode())
	}
	if dirItem.Path() != dirPath {
		t.Errorf("DirectoryItem path mismatch: expected %s, got %s", dirPath, dirItem.Path())
	}
}
