package synthfs_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestUnarchiveItem(t *testing.T) {
	t.Run("NewUnarchive creates correct item", func(t *testing.T) {
		item := synthfs.NewUnarchive("test.tar.gz", "extract/")

		if item.ArchivePath() != "test.tar.gz" {
			t.Errorf("Expected archive path 'test.tar.gz', got '%s'", item.ArchivePath())
		}

		if item.ExtractPath() != "extract/" {
			t.Errorf("Expected extract path 'extract/', got '%s'", item.ExtractPath())
		}

		if item.Type() != "unarchive" {
			t.Errorf("Expected type 'unarchive', got '%s'", item.Type())
		}

		if item.Path() != "test.tar.gz" {
			t.Errorf("Expected path 'test.tar.gz', got '%s'", item.Path())
		}

		if len(item.Patterns()) != 0 {
			t.Errorf("Expected empty patterns, got %v", item.Patterns())
		}

		if item.Overwrite() != false {
			t.Errorf("Expected overwrite false, got %v", item.Overwrite())
		}
	})

	t.Run("WithPatterns sets patterns correctly", func(t *testing.T) {
		item := synthfs.NewUnarchive("test.zip", "extract/").
			WithPatterns("*.txt", "docs/**")

		patterns := item.Patterns()
		if len(patterns) != 2 {
			t.Errorf("Expected 2 patterns, got %d", len(patterns))
		}

		if patterns[0] != "*.txt" || patterns[1] != "docs/**" {
			t.Errorf("Expected patterns ['*.txt', 'docs/**'], got %v", patterns)
		}
	})

	t.Run("WithOverwrite sets overwrite correctly", func(t *testing.T) {
		item := synthfs.NewUnarchive("test.zip", "extract/").
			WithOverwrite(true)

		if !item.Overwrite() {
			t.Errorf("Expected overwrite true, got %v", item.Overwrite())
		}
	})
}
