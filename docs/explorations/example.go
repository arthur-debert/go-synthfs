package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func main() {
	fmt.Println("🔧 SynthFS Usage Example")
	fmt.Println("========================")

	// Create a temporary directory for our example
	tempDir, err := os.MkdirTemp("", "synthfs-example-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("Warning: failed to remove temp dir: %v\n", err)
		}
	}()

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

// Example 1: Demonstrates the new imperative Batch API
func simpleFileOperations(baseDir string) {
	fmt.Println("  🎯 Using new Batch API...")

	// Create a filesystem and batch
	fs := synthfs.NewOSFileSystem(".")
	batch := synthfs.NewBatch().WithFileSystem(fs)

	// Add operations using the imperative API
	_, err := batch.CreateDir(filepath.Join(baseDir, "data"))
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateDir: %v\n", err)
		return
	}

	_, err = batch.CreateFile(filepath.Join(baseDir, "config.json"),
		[]byte(`{"version": "1.0", "debug": true}`))
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateFile: %v\n", err)
		return
	}

	fmt.Printf("  📋 Created batch with %d operations\n", len(batch.Operations()))

	// Execute the batch
	fmt.Println("  🚀 Executing batch...")
	result, err := batch.Execute()
	if err != nil {
		fmt.Printf("  ❌ Execution failed: %v\n", err)
		return
	}

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

// Example 2: More complex workflow using new Batch API
func complexWorkflow(baseDir string) {
	fmt.Println("  🎯 Building complex workflow with Batch API...")

	workDir := filepath.Join(baseDir, "workflow")
	sourceFile := filepath.Join(workDir, "source.txt")
	backupFile := filepath.Join(workDir, "backup", "source.txt")
	processedFile := filepath.Join(workDir, "processed.txt")

	// Create a complex workflow using Batch API
	fs := synthfs.NewOSFileSystem(".")
	batch := synthfs.NewBatch().WithFileSystem(fs)

	// 1. Setup: Create directory structure and initial file
	_, err := batch.CreateDir(workDir)
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateDir: %v\n", err)
		return
	}

	_, err = batch.CreateDir(filepath.Join(workDir, "backup"))
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateDir: %v\n", err)
		return
	}

	_, err = batch.CreateFile(sourceFile, []byte("Original data for processing"))
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateFile: %v\n", err)
		return
	}

	// 2. Backup: Copy original file to backup location
	_, err = batch.Copy(sourceFile, backupFile)
	if err != nil {
		fmt.Printf("  ❌ Failed to add Copy: %v\n", err)
		return
	}

	// 3. Process: Move file to new location (simulating processing)
	_, err = batch.Move(sourceFile, processedFile)
	if err != nil {
		fmt.Printf("  ❌ Failed to add Move: %v\n", err)
		return
	}

	fmt.Printf("  📋 Created batch with %d operations\n", len(batch.Operations()))

	// Execute the workflow
	fmt.Println("  🚀 Executing complex workflow...")
	result, err := batch.Execute()
	if err != nil {
		fmt.Printf("  ❌ Execution failed: %v\n", err)
		return
	}

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
		fmt.Println("  🔄 Attempting rollback...")
		ctx := context.Background()
		if err := result.Rollback(ctx); err != nil {
			fmt.Printf("  ❌ Rollback failed: %v\n", err)
		} else {
			fmt.Println("  ✅ Rollback completed")
		}
	}
}

// Example 3: Demonstrates error handling with Batch API
func errorHandlingExample(baseDir string) {
	fmt.Println("  🎯 Demonstrating error handling with Batch API...")

	fs := synthfs.NewOSFileSystem(".")
	batch := synthfs.NewBatch().WithFileSystem(fs)

	// Create operations that should work
	validDir := filepath.Join(baseDir, "valid")
	_, err := batch.CreateDir(validDir)
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateDir: %v\n", err)
		return
	}

	_, err = batch.CreateFile(filepath.Join(validDir, "test.txt"), []byte("test content"))
	if err != nil {
		fmt.Printf("  ❌ Failed to add CreateFile: %v\n", err)
		return
	}

	// Try to add an operation that will fail validation (empty path)
	_, err = batch.CreateFile("", []byte("this will fail"))
	if err != nil {
		fmt.Printf("  ✅ Validation correctly caught error: %v\n", err)
		fmt.Println("  💡 This demonstrates upfront validation - no filesystem changes were made")
		return
	}

	// If validation somehow passed, show execution results
	fmt.Printf("  📋 Created batch with %d operations\n", len(batch.Operations()))

	fmt.Println("  🚀 Executing batch...")
	result, err := batch.Execute()
	if err != nil {
		fmt.Printf("  ✅ Execution correctly failed: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("  ✅ Execution correctly failed: %v\n", result.Errors)

		// Show rollback capabilities
		fmt.Println("  🔄 Rolling back any successful operations...")
		ctx := context.Background()
		if err := result.Rollback(ctx); err != nil {
			fmt.Printf("  ❌ Rollback failed: %v\n", err)
		} else {
			fmt.Println("  ✅ Rollback completed - filesystem restored")
		}
	}
}
