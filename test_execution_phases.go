package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func main() {
	// Create a test filesystem
	testDir := "/tmp/synthfs_test"
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	fmt.Println("Testing execution phases implementation...")

	// Test 1: Phase 1-3 - Prerequisites working
	fmt.Println("\n=== Test 1: Prerequisites Resolution ===")
	if err := testPrerequisites(testDir); err != nil {
		log.Fatal("Prerequisites test failed:", err)
	}
	fmt.Println("✅ Prerequisites resolution working correctly")

	// Test 2: Phase 4-6 - SimpleBatch behavior
	fmt.Println("\n=== Test 2: SimpleBatch Behavior ===")
	if err := testSimpleBatch(testDir); err != nil {
		log.Fatal("SimpleBatch test failed:", err)
	}
	fmt.Println("✅ SimpleBatch behavior working correctly")

	// Test 3: Legacy behavior still works
	fmt.Println("\n=== Test 3: Legacy Behavior ===")
	if err := testLegacyBehavior(testDir); err != nil {
		log.Fatal("Legacy behavior test failed:", err)
	}
	fmt.Println("✅ Legacy behavior working correctly")

	fmt.Println("\n🎉 All execution phases working correctly!")
}

func testPrerequisites(testDir string) error {
	// Create filesystem
	fs := filesystem.NewOSFileSystem(testDir)
	registry := synthfs.GetDefaultRegistry()
	
	// Test prerequisite resolution
	batchImpl := batch.NewBatch(fs, registry)
	
	// Create a file in a nested directory that doesn't exist
	deepFile := filepath.Join("level1", "level2", "level3", "test.txt")
	_, err := batchImpl.CreateFile(deepFile, []byte("test content"))
	if err != nil {
		return fmt.Errorf("failed to create file operation: %w", err)
	}
	
	// Run with prerequisite resolution
	result, err := batchImpl.RunWithPrerequisites()
	if err != nil {
		return fmt.Errorf("failed to run with prerequisites: %w", err)
	}
	
	if !result.IsSuccess() {
		return fmt.Errorf("batch execution failed")
	}
	
	// Check if parent directories were created
	expectedDirs := []string{
		filepath.Join(testDir, "level1"),
		filepath.Join(testDir, "level1", "level2"),
		filepath.Join(testDir, "level1", "level2", "level3"),
	}
	
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("expected directory %s was not created", dir)
		}
	}
	
	// Check if file was created
	expectedFile := filepath.Join(testDir, deepFile)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		return fmt.Errorf("expected file %s was not created", expectedFile)
	}
	
	return nil
}

func testSimpleBatch(testDir string) error {
	// Clean up first
	os.RemoveAll(filepath.Join(testDir, "simple_test"))
	
	// Create filesystem
	fs := filesystem.NewOSFileSystem(testDir)
	registry := synthfs.GetDefaultRegistry()
	
	// Test SimpleBatch behavior (should be default)
	batchImpl := batch.NewBatch(fs, registry)
	
	// Create a file in a nested directory
	deepFile := filepath.Join("simple_test", "nested", "file.txt")
	_, err := batchImpl.CreateFile(deepFile, []byte("simple test content"))
	if err != nil {
		return fmt.Errorf("failed to create file operation: %w", err)
	}
	
	// Run normally (should use prerequisite resolution by default)
	result, err := batchImpl.Run()
	if err != nil {
		return fmt.Errorf("failed to run batch: %w", err)
	}
	
	if !result.IsSuccess() {
		return fmt.Errorf("batch execution failed")
	}
	
	// Check if file was created
	expectedFile := filepath.Join(testDir, deepFile)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		return fmt.Errorf("expected file %s was not created", expectedFile)
	}
	
	return nil
}

func testLegacyBehavior(testDir string) error {
	// Clean up first
	os.RemoveAll(filepath.Join(testDir, "legacy_test"))
	
	// Create filesystem
	fs := filesystem.NewOSFileSystem(testDir)
	registry := synthfs.GetDefaultRegistry()
	
	// Test legacy behavior
	batchImpl := batch.NewBatchWithLegacyBehavior(fs, registry)
	
	// Create a file in a nested directory
	deepFile := filepath.Join("legacy_test", "nested", "file.txt")
	_, err := batchImpl.CreateFile(deepFile, []byte("legacy test content"))
	if err != nil {
		return fmt.Errorf("failed to create file operation: %w", err)
	}
	
	// Run normally (should use legacy behavior with auto parent creation)
	result, err := batchImpl.Run()
	if err != nil {
		return fmt.Errorf("failed to run batch: %w", err)
	}
	
	if !result.IsSuccess() {
		return fmt.Errorf("batch execution failed")
	}
	
	// Check if file was created
	expectedFile := filepath.Join(testDir, deepFile)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		return fmt.Errorf("expected file %s was not created", expectedFile)
	}
	
	return nil
}