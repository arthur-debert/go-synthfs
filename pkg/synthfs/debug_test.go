package synthfs_test

import (
	"testing"
	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestDebugCopyOperation(t *testing.T) {
	testFS := synthfs.NewTestFileSystem()
	
	// Create source file
	err := testFS.WriteFile("source.txt", []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	
	registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
	
	// Try to copy
	op, err := batch.Copy("source.txt", "dest.txt")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	
	t.Logf("Operation created: %v", op)
	t.Logf("Operation ID: %v", op.ID())
	t.Logf("Operation description: %v", op.(synthfs.Operation).Describe())
}