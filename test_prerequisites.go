package main

import (
	"fmt"
	"log"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func main() {
	// Create a test operation
	op := operations.NewCreateFileOperation(
		core.OperationID("test-file"),
		"nested/dir/file.txt",
	)

	// Test Prerequisites method
	prereqs := op.Prerequisites()
	
	fmt.Printf("Operation: %s\n", op.Describe().Type)
	fmt.Printf("Path: %s\n", op.Describe().Path)
	fmt.Printf("Prerequisites count: %d\n", len(prereqs))
	
	for i, prereq := range prereqs {
		fmt.Printf("  Prerequisite %d: %s for path %s\n", i+1, prereq.Type(), prereq.Path())
	}

	// Test with SimpleBatch
	testFS := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch().WithFileSystem(testFS)
	
	// Create a nested file - this should trigger prerequisite resolution
	_, err := batch.CreateFile("deep/nested/path/file.txt", []byte("test content"))
	if err != nil {
		log.Printf("Error creating file: %v", err)
		return
	}
	
	// Check operations created
	ops := batch.Operations()
	fmt.Printf("\nBatch operations count: %d\n", len(ops))
	
	for i, op := range ops {
		if desc, ok := op.(interface{ Describe() core.OperationDesc }); ok {
			fmt.Printf("  Operation %d: %s at %s\n", i+1, desc.Describe().Type, desc.Describe().Path)
		}
	}
	
	// Test with prerequisite resolution enabled
	opts := synthfs.PipelineOptions{
		ResolvePrerequisites: true,
	}
	
	result, err := batch.RunWithOptions(opts)
	if err != nil {
		log.Printf("Error running batch: %v", err)
		return
	}
	
	fmt.Printf("\nExecution result: Success=%v, Operations=%d\n", result.Success, len(result.Operations))
	
	// Check if file was created
	if _, err := testFS.Stat("deep/nested/path/file.txt"); err == nil {
		fmt.Println("✓ File was created successfully!")
	} else {
		fmt.Printf("✗ File creation failed: %v\n", err)
	}
	
	// Check if parent directories were created
	if _, err := testFS.Stat("deep/nested/path"); err == nil {
		fmt.Println("✓ Parent directories were created successfully!")
	} else {
		fmt.Printf("✗ Parent directory creation failed: %v\n", err)
	}
}