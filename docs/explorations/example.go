package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/ops"
)

func main() {
	fmt.Println("🔧 SynthFS Usage Example")
	fmt.Println("========================")

	// Create a temporary directory for our example
	tempDir, err := os.MkdirTemp("", "synthfs-example-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("📁 Working in: %s\n\n", tempDir)

	// Example 1: Simple File Operations
	fmt.Println("📝 Example 1: Simple File Operations")
	simpleFileOperations(tempDir)

	// Example 2: Complex Workflow with Dependencies
	fmt.Println("\n🔄 Example 2: Complex Workflow")
	complexWorkflow(tempDir)

	// Example 3: Error Handling and Rollback
	fmt.Println("\n❌ Example 3: Error Handling and Rollback")
	errorHandlingExample(tempDir)

	fmt.Println("\n✅ All examples completed!")
}

// Example 1: Demonstrates the core synthfs model - queue, validate, execute
func simpleFileOperations(baseDir string) {
	// Step 1: Create operations (the "reverse receipts")
	fmt.Println("  🎯 Creating operations...")

	// Create different types of filesystem items
	configFile := synthfs.NewFile(filepath.Join(baseDir, "config.json")).
		WithContent([]byte(`{"version": "1.0", "debug": true}`)).
		WithMode(0644)

	dataDir := synthfs.NewDirectory(filepath.Join(baseDir, "data")).
		WithMode(0755)

	// Step 2: Queue operations
	queue := synthfs.NewMemQueue()

	// Add operations to queue (order matters due to dependencies)
	queue.Add(ops.Create(dataDir))    // Create directory first
	queue.Add(ops.Create(configFile)) // Then create file

	fmt.Printf("  📋 Queued %d operations\n", len(queue.Operations()))

	// Step 3: Validate upfront (before any changes)
	ctx := context.Background()
	fs := synthfs.NewOSFileSystem(".")

	fmt.Println("  🔍 Validating operations...")
	if err := queue.Validate(ctx, fs); err != nil {
		fmt.Printf("  ❌ Validation failed: %v\n", err)
		return
	}
	fmt.Println("  ✅ All operations valid")

	// Step 4: Execute later
	fmt.Println("  🚀 Executing operations...")
	executor := synthfs.NewExecutor()
	result := executor.Execute(ctx, queue, fs)

	if result.Success {
		fmt.Printf("  ✅ Successfully executed %d operations\n", len(result.Operations))
		fmt.Printf("  ⏱️  Total time: %v\n", result.Duration)

		// Verify results
		if _, err := os.Stat(filepath.Join(baseDir, "data")); err == nil {
			fmt.Println("  📁 Directory 'data' created successfully")
		}
		if _, err := os.Stat(filepath.Join(baseDir, "config.json")); err == nil {
			fmt.Println("  📄 File 'config.json' created successfully")
		}
	} else {
		fmt.Printf("  ❌ Execution failed: %v\n", result.Errors)
	}
}

// Example 2: More complex workflow showing unified API for different operations
func complexWorkflow(baseDir string) {
	fmt.Println("  🎯 Building complex operation workflow...")

	workDir := filepath.Join(baseDir, "workflow")
	sourceFile := filepath.Join(workDir, "source.txt")
	backupFile := filepath.Join(workDir, "backup", "source.txt")
	processedFile := filepath.Join(workDir, "processed.txt")

	// Create a complex workflow: setup → backup → process → cleanup
	queue := synthfs.NewMemQueue()

	// 1. Setup: Create directory structure and initial file
	queue.Add(ops.Create(synthfs.NewDirectory(workDir).WithMode(0755)))
	queue.Add(ops.Create(synthfs.NewDirectory(filepath.Join(workDir, "backup")).WithMode(0755)))
	queue.Add(ops.Create(synthfs.NewFile(sourceFile).
		WithContent([]byte("Original data for processing")).
		WithMode(0644)))

	// 2. Backup: Copy original file to backup location
	queue.Add(ops.Copy(sourceFile, backupFile))

	// 3. Process: Move file to new location (simulating processing)
	queue.Add(ops.Move(sourceFile, processedFile))

	// 4. Cleanup: Remove backup directory (would contain backup file)
	// Note: In a real scenario, you might keep backups longer

	fmt.Printf("  📋 Queued %d operations for complex workflow\n", len(queue.Operations()))

	// Execute the workflow
	ctx := context.Background()
	fs := synthfs.NewOSFileSystem(".")
	executor := synthfs.NewExecutor()

	fmt.Println("  🔍 Validating complex workflow...")
	if err := queue.Validate(ctx, fs); err != nil {
		fmt.Printf("  ❌ Validation failed: %v\n", err)
		return
	}

	fmt.Println("  🚀 Executing complex workflow...")
	result := executor.Execute(ctx, queue, fs)

	if result.Success {
		fmt.Printf("  ✅ Complex workflow completed in %v\n", result.Duration)

		// Show what was accomplished
		if _, err := os.Stat(backupFile); err == nil {
			fmt.Println("  💾 Backup created successfully")
		}
		if _, err := os.Stat(processedFile); err == nil {
			fmt.Println("  🔄 File moved to processed location")
		}
		if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
			fmt.Println("  🗑️  Original file moved (no longer at source)")
		}
	} else {
		fmt.Printf("  ❌ Workflow failed: %v\n", result.Errors)

		// Demonstrate rollback capability
		if result.Rollback != nil {
			fmt.Println("  🔄 Attempting rollback...")
			if err := result.Rollback(ctx); err != nil {
				fmt.Printf("  ❌ Rollback failed: %v\n", err)
			} else {
				fmt.Println("  ✅ Rollback completed")
			}
		}
	}
}

// Example 3: Demonstrates error handling and rollback
func errorHandlingExample(baseDir string) {
	fmt.Println("  🎯 Demonstrating error handling...")

	queue := synthfs.NewMemQueue()

	// Create operations that should work
	validDir := filepath.Join(baseDir, "valid")
	queue.Add(ops.Create(synthfs.NewDirectory(validDir).WithMode(0755)))
	queue.Add(ops.Create(synthfs.NewFile(filepath.Join(validDir, "test.txt")).
		WithContent([]byte("test content")).
		WithMode(0644)))

	// Add an operation that will fail (invalid path)
	invalidFile := "" // Empty path should fail validation
	queue.Add(ops.Create(synthfs.NewFile(invalidFile).
		WithContent([]byte("this will fail")).
		WithMode(0644)))

	fmt.Printf("  📋 Created queue with %d operations (1 intentionally invalid)\n", len(queue.Operations()))

	ctx := context.Background()
	fs := synthfs.NewOSFileSystem(".")

	// This should fail during validation
	fmt.Println("  🔍 Validating operations (expecting failure)...")
	if err := queue.Validate(ctx, fs); err != nil {
		fmt.Printf("  ✅ Validation correctly caught error: %v\n", err)
		fmt.Println("  💡 This demonstrates upfront validation - no filesystem changes were made")
		return
	}

	// If validation somehow passed, execution would fail
	executor := synthfs.NewExecutor()
	result := executor.Execute(ctx, queue, fs)

	if !result.Success {
		fmt.Printf("  ✅ Execution correctly failed: %v\n", result.Errors)

		// Show rollback capabilities
		if result.Rollback != nil {
			fmt.Println("  🔄 Rolling back any successful operations...")
			if err := result.Rollback(ctx); err != nil {
				fmt.Printf("  ❌ Rollback failed: %v\n", err)
			} else {
				fmt.Println("  ✅ Rollback completed - filesystem restored")
			}
		}
	}
}
