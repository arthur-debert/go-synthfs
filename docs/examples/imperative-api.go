package main

import (
	"context"
	"fmt"
	"log"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

// Example demonstrating the new imperative Batch API
func main() {
	// Create a batch with validate-as-you-go
	batch := synthfs.NewBatch()

	// Simple usage - each operation validates immediately
	fmt.Println("=== Simple Batch Operations ===")

	// Create a directory
	_, err := batch.CreateDir("project")
	if err != nil {
		log.Fatalf("CreateDir failed: %v", err)
	}
	fmt.Println("✓ Added CreateDir operation for 'project'")

	// Create a file - this will auto-create parent directories if needed
	_, err = batch.CreateFile("project/config.yaml", []byte("version: 1.0\nname: my-app"))
	if err != nil {
		log.Fatalf("CreateFile failed: %v", err)
	}
	fmt.Println("✓ Added CreateFile operation for 'project/config.yaml'")

	// Copy a file
	_, err = batch.Copy("project/config.yaml", "project/config.backup.yaml")
	if err != nil {
		log.Fatalf("Copy failed: %v", err)
	}
	fmt.Println("✓ Added Copy operation")

	// Advanced usage - nested paths with auto-dependency resolution
	fmt.Println("\n=== Auto-Dependency Resolution ===")

	// This single call will automatically create:
	// 1. project/src directory
	// 2. project/src/components directory
	// 3. project/src/components/main.go file
	_, err = batch.CreateFile("project/src/components/main.go", []byte("package main\n\nfunc main() {\n\tfmt.Println(\"Hello World\")\n}"))
	if err != nil {
		log.Fatalf("Nested CreateFile failed: %v", err)
	}
	fmt.Println("✓ Added nested CreateFile - auto-resolved dependencies")

	// Show what operations were generated
	operations := batch.Operations()
	fmt.Printf("\n=== Generated Operations (%d total) ===\n", len(operations))
	for i, op := range operations {
		desc := op.Describe()
		fmt.Printf("%d. %s: %s\n", i+1, desc.Type, desc.Path)
	}

	// Execute the batch
	fmt.Println("\n=== Executing Batch ===")
	result, err := batch.Execute()
	if err != nil {
		log.Fatalf("Batch execution failed: %v", err)
	}

	// Show results
	fmt.Printf("✅ Batch executed successfully!\n")
	fmt.Printf("   - %d operations completed\n", len(result.Operations))
	fmt.Printf("   - Execution time: %v\n", result.Duration)
	fmt.Printf("   - Success: %v\n", result.Success)

	// Show individual operation results
	fmt.Println("\n=== Operation Results ===")
	for i, opResult := range result.Operations {
		desc := opResult.Operation.Describe()
		fmt.Printf("%d. %s %s -> %s (%v)\n",
			i+1,
			desc.Type,
			desc.Path,
			opResult.Status,
			opResult.Duration)
	}

	// Rollback is available if needed
	if !result.Success {
		fmt.Println("\n=== Rolling Back ===")
		if err := result.Rollback(context.Background()); err != nil {
			log.Printf("Rollback failed: %v", err)
		} else {
			fmt.Println("✓ Rollback completed")
		}
	}
}
