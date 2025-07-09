package batch_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// TestPrerequisiteDeclaration verifies that operations properly declare prerequisites
func TestPrerequisiteDeclaration(t *testing.T) {
	t.Run("Operations declare expected prerequisites", func(t *testing.T) {
		fs := synthfs.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()
		b := batch.NewBatch(fs, registry)

		// Create a file operation
		fileOp, err := b.CreateFile("subdir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Check that the operation declares prerequisites
		if prerequisiteGetter, ok := fileOp.(interface{ Prerequisites() []core.Prerequisite }); ok {
			prereqs := prerequisiteGetter.Prerequisites()

			if len(prereqs) != 2 {
				t.Errorf("Expected 2 prerequisites for file creation, got %d", len(prereqs))
			}

			// Check for parent directory prerequisite
			hasParentDir := false
			hasNoConflict := false

			for _, prereq := range prereqs {
				switch prereq.Type() {
				case "parent_dir":
					hasParentDir = true
				case "no_conflict":
					hasNoConflict = true
				}
			}

			if !hasParentDir {
				t.Error("File operation should declare parent_dir prerequisite")
			}

			if !hasNoConflict {
				t.Error("File operation should declare no_conflict prerequisite")
			}
		} else {
			t.Error("Operation should implement Prerequisites method")
		}
	})
}
