package targets_test

import (
	"io/fs"
	"reflect"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

func TestFileItem(t *testing.T) {
	filePath := "/tmp/testfile.txt"
	fileContent := []byte("hello world")
	fileMode := fs.FileMode(0600)

	file := targets.NewFile(filePath).
		WithContent(fileContent).
		WithMode(fileMode)

	if file.Path() != filePath {
		t.Errorf("Expected path %s, got %s", filePath, file.Path())
	}
	if file.Type() != "file" {
		t.Errorf("Expected type 'file', got %s", file.Type())
	}
	if !reflect.DeepEqual(file.Content(), fileContent) {
		t.Errorf("Expected content %v, got %v", fileContent, file.Content())
	}
	if file.Mode() != fileMode {
		t.Errorf("Expected mode %v, got %v", fileMode, file.Mode())
	}

	// Test default mode
	defaultModeFile := targets.NewFile("default.txt")
	if defaultModeFile.Mode() != 0644 {
		t.Errorf("Expected default mode 0644, got %v", defaultModeFile.Mode())
	}
}
