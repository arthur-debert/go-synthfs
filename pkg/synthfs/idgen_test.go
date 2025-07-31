package synthfs

import (
	"strings"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestIDGenerators(t *testing.T) {
	t.Run("HashIDGenerator", func(t *testing.T) {
		id1 := HashIDGenerator("create_file", "/path/to/file")
		// Small delay to ensure different timestamp
		time.Sleep(1 * time.Nanosecond)
		id2 := HashIDGenerator("create_file", "/path/to/file")
		
		// Should generate different IDs for same inputs (due to timestamp)
		if id1 == id2 {
			t.Error("HashIDGenerator should generate unique IDs")
		}
		
		// Should start with operation type
		if !strings.HasPrefix(string(id1), "create_file-") {
			t.Errorf("ID should start with operation type, got: %s", id1)
		}
		
		// Should have consistent format
		parts := strings.Split(string(id1), "-")
		if len(parts) != 2 {
			t.Errorf("ID should have format 'type-hash', got: %s", id1)
		}
		if len(parts[1]) != 8 {
			t.Errorf("Hash part should be 8 characters, got: %s", parts[1])
		}
	})
	
	t.Run("SequenceIDGenerator", func(t *testing.T) {
		ResetSequenceCounter()
		
		id1 := SequenceIDGenerator("create_file", "/path1")
		id2 := SequenceIDGenerator("create_dir", "/path2")
		id3 := SequenceIDGenerator("delete", "/path3")
		
		// Should generate sequential IDs
		if id1 != "create_file-1" {
			t.Errorf("Expected 'create_file-1', got: %s", id1)
		}
		if id2 != "create_dir-2" {
			t.Errorf("Expected 'create_dir-2', got: %s", id2)
		}
		if id3 != "delete-3" {
			t.Errorf("Expected 'delete-3', got: %s", id3)
		}
	})
	
	t.Run("TimestampIDGenerator", func(t *testing.T) {
		id1 := TimestampIDGenerator("create_file", "/path/to/file")
		time.Sleep(1 * time.Millisecond) // Ensure different timestamp
		id2 := TimestampIDGenerator("create_file", "/path/to/file")
		
		// Should generate different IDs
		if id1 == id2 {
			t.Error("TimestampIDGenerator should generate unique IDs")
		}
		
		// Should start with operation type
		if !strings.HasPrefix(string(id1), "create_file-") {
			t.Errorf("ID should start with operation type, got: %s", id1)
		}
	})
}

func TestSetIDGenerator(t *testing.T) {
	// Save original generator
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	
	// Test custom generator
	customGen := func(opType, path string) core.OperationID {
		return core.OperationID("custom-" + opType)
	}
	
	SetIDGenerator(customGen)
	
	id := GenerateID("test_op", "/path")
	if id != "custom-test_op" {
		t.Errorf("Expected 'custom-test_op', got: %s", id)
	}
}

func TestGenerateID(t *testing.T) {
	// Save original generator
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	
	// Use sequence generator for predictable results
	SetIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	
	id1 := GenerateID("create_file", "/path1")
	id2 := GenerateID("create_dir", "/path2")
	
	if id1 != "create_file-1" {
		t.Errorf("Expected 'create_file-1', got: %s", id1)
	}
	if id2 != "create_dir-2" {
		t.Errorf("Expected 'create_dir-2', got: %s", id2)
	}
}