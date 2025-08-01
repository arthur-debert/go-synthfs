package batch_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

// TestErrorHandling verifies error handling with prerequisite resolution
func TestErrorHandling(t *testing.T) {
	t.Run("Batch handles prerequisite validation errors", func(t *testing.T) {
		fs := testutil.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()
		b := batch.NewBatch(fs, registry)

		// Try to copy from non-existent source
		_, err := b.Copy("nonexistent.txt", "dest/copy.txt")
		if err == nil {
			t.Fatalf("Expected an error when adding a Copy operation with a non-existent source")
		}
	})
}
