package synthfs_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestDebugCopyOperation(t *testing.T) {
	testFS := testutil.NewTestFileSystem()

	// Create source file
	err := testFS.WriteFile("source.txt", []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	registry := synthfs.GetDefaultRegistry()
	fs := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

	// Try to copy
	op, err := batch.Copy("source.txt", "dest.txt")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	t.Logf("Operation created: %v", op)
	t.Logf("Operation ID: %v", op.(synthfs.Operation).ID())
	t.Logf("Operation description: %v", op.(synthfs.Operation).Describe())
}

func TestDebugCreateFileOperation(t *testing.T) {
	testFS := testutil.NewTestFileSystem()
	registry := synthfs.GetDefaultRegistry()
	fs := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

	// Create a file in nested directory
	op, err := batch.CreateFile("nested/dir/file.txt", []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	t.Logf("CreateFile operation created: %v", op)
	t.Logf("Operation ID: %v", op.(synthfs.Operation).ID())
	t.Logf("Operation description: %v", op.(synthfs.Operation).Describe())

	// Check if operation has item
	if itemGetter, ok := op.(interface{ GetItem() interface{} }); ok {
		item := itemGetter.GetItem()
		t.Logf("Operation has item: %v", item)
		if item != nil {
			t.Logf("Item type: %T", item)
		}
	}

	// Check prerequisites
	if prereqGetter, ok := op.(interface{ Prerequisites() []interface{} }); ok {
		prereqs := prereqGetter.Prerequisites()
		t.Logf("Operation has %d prerequisites", len(prereqs))
		for i, prereq := range prereqs {
			t.Logf("Prerequisite %d: %v", i, prereq)
		}
	}

	// Execute the batch
	result, err := batch.Run()
	if err != nil {
		t.Fatalf("Batch execution failed: %v", err)
	}

	t.Logf("Batch execution success: %v", result.IsSuccess())
	if !result.IsSuccess() {
		t.Logf("Batch error: %v", result.GetError())
	}

	// Check operations in result
	ops := result.GetOperations()
	t.Logf("Result contains %d operations", len(ops))
	for i, op := range ops {
		t.Logf("Result operation %d: %v", i, op)
	}

	// Check if file was created
	if testutil.FileExists(t, testFS, "nested/dir/file.txt") {
		t.Log("SUCCESS: File was created!")
	} else {
		t.Error("FAILED: File was not created")

		// Check what exists
		t.Log("Checking filesystem contents...")
		if testutil.FileExists(t, testFS, "nested") {
			t.Log("Directory 'nested' exists")
		}
		if testutil.FileExists(t, testFS, "nested/dir") {
			t.Log("Directory 'nested/dir' exists")
		}
	}
}
