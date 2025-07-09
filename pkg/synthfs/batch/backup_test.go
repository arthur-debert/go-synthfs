package batch_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// TestBatchWithBackup verifies that batch works with backup/restore functionality
func TestBatchWithBackup(t *testing.T) {
	t.Run("Batch can run with backup enabled", func(t *testing.T) {
		fs := synthfs.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()

		// Create existing file
		synthfs.CreateTestFile(t, fs, "existing.txt", []byte("original"))

		b := batch.NewBatch(fs, registry)

		// Overwrite existing file
		_, err := b.CreateFile("existing.txt", []byte("new content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}

		// Run with backup enabled
		executor := synthfs.NewExecutor()
		pipeline := synthfs.NewMemPipeline()
		for _, op := range b.Operations() {
			if err := pipeline.Add(op.(synthfs.Operation)); err != nil {
				t.Fatalf("Failed to add operation to pipeline: %v", err)
			}
		}
		result := executor.RunWithOptions(context.Background(), pipeline, fs, core.PipelineOptions{
			Restorable: true,
		})

		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}

		// Verify file was overwritten
		synthfs.AssertFileContent(t, fs, "existing.txt", []byte("new content"))

		// Verify restore operations were created
		if len(result.GetRestoreOps()) == 0 {
			t.Error("No restore operations were created")
		}
	})
}
