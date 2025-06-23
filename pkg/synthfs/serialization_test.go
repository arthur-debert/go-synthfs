package synthfs_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestOperationPlan(t *testing.T) {
	t.Run("NewOperationPlan", func(t *testing.T) {
		description := "Test plan"
		plan := synthfs.NewOperationPlan(description)

		if plan.Metadata.Description != description {
			t.Errorf("Expected description %q, got %q", description, plan.Metadata.Description)
		}

		if plan.Metadata.Version != "1.0" {
			t.Errorf("Expected version '1.0', got %q", plan.Metadata.Version)
		}

		if len(plan.Operations) != 0 {
			t.Errorf("Expected empty operations list, got %d operations", len(plan.Operations))
		}
	})

	t.Run("AddOperation", func(t *testing.T) {
		plan := synthfs.NewOperationPlan("Test")
		op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)

		plan.AddOperation(op)

		if len(plan.Operations) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(plan.Operations))
		}

		if plan.Operations[0].ID() != op.ID() {
			t.Errorf("Expected operation ID %q, got %q", op.ID(), plan.Operations[0].ID())
		}
	})

	t.Run("ToQueue", func(t *testing.T) {
		plan := synthfs.NewOperationPlan("Test")
		op1 := synthfs.NewSerializableCreateFile("test1.txt", []byte("content1"), 0644)
		op2 := synthfs.NewSerializableCreateFile("test2.txt", []byte("content2"), 0644)

		plan.AddOperation(op1)
		plan.AddOperation(op2)

		queue := plan.ToQueue()
		ops := queue.Operations()

		if len(ops) != 2 {
			t.Errorf("Expected 2 operations in queue, got %d", len(ops))
		}
	})
}

func TestMarshalUnmarshalPlan(t *testing.T) {
	t.Run("Round trip", func(t *testing.T) {
		// Create a plan
		originalPlan := synthfs.NewOperationPlan("Test plan")
		op1 := synthfs.NewSerializableCreateFile("file1.txt", []byte("content1"), 0644)
		op2 := synthfs.NewSerializableCreateFile("file2.txt", []byte("content2"), 0755)
		op2.WithDependency(op1.ID())

		originalPlan.AddOperation(op1)
		originalPlan.AddOperation(op2)

		// Marshal to JSON
		data, err := synthfs.MarshalPlan(originalPlan)
		if err != nil {
			t.Fatalf("MarshalPlan failed: %v", err)
		}

		// Verify it's valid JSON
		var jsonData map[string]interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			t.Fatalf("Generated JSON is invalid: %v", err)
		}

		// Unmarshal back
		unmarshaledPlan, err := synthfs.UnmarshalPlan(data)
		if err != nil {
			t.Fatalf("UnmarshalPlan failed: %v", err)
		}

		// Verify metadata
		if unmarshaledPlan.Metadata.Description != originalPlan.Metadata.Description {
			t.Errorf("Description mismatch: expected %q, got %q",
				originalPlan.Metadata.Description, unmarshaledPlan.Metadata.Description)
		}

		if unmarshaledPlan.Metadata.Version != originalPlan.Metadata.Version {
			t.Errorf("Version mismatch: expected %q, got %q",
				originalPlan.Metadata.Version, unmarshaledPlan.Metadata.Version)
		}

		// Verify operations
		if len(unmarshaledPlan.Operations) != len(originalPlan.Operations) {
			t.Errorf("Operations count mismatch: expected %d, got %d",
				len(originalPlan.Operations), len(unmarshaledPlan.Operations))
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		invalidJSON := []byte(`{"invalid": json}`)
		_, err := synthfs.UnmarshalPlan(invalidJSON)
		if err == nil {
			t.Errorf("Expected UnmarshalPlan to fail with invalid JSON")
		}
	})
}

func TestSerializableCreateFileOperation(t *testing.T) {
	t.Run("NewSerializableCreateFile", func(t *testing.T) {
		path := "test.txt"
		content := []byte("test content")
		mode := fs.FileMode(0644)

		op := synthfs.NewSerializableCreateFile(path, content, mode)

		if !strings.Contains(string(op.ID()), path) {
			t.Errorf("Expected ID to contain path %q, got %q", path, op.ID())
		}

		desc := op.Describe()
		if desc.Type != "create_file" {
			t.Errorf("Expected type 'create_file', got %q", desc.Type)
		}

		if desc.Path != path {
			t.Errorf("Expected path %q, got %q", path, desc.Path)
		}
	})

	t.Run("WithID", func(t *testing.T) {
		op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
		customID := synthfs.OperationID("custom-id")

		op.WithID(customID)

		if op.ID() != customID {
			t.Errorf("Expected ID %q, got %q", customID, op.ID())
		}
	})

	t.Run("WithDependency", func(t *testing.T) {
		op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
		depID := synthfs.OperationID("dependency-id")

		op.WithDependency(depID)

		deps := op.Dependencies()
		if len(deps) != 1 {
			t.Errorf("Expected 1 dependency, got %d", len(deps))
		}

		if deps[0] != depID {
			t.Errorf("Expected dependency %q, got %q", depID, deps[0])
		}
	})

	t.Run("Execute", func(t *testing.T) {
		content := []byte("test content")
		op := synthfs.NewSerializableCreateFile("test.txt", content, 0644)

		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		err := op.Execute(ctx, tfs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file was created
		file, err := tfs.Open("test.txt")
		if err != nil {
			t.Fatalf("Failed to open created file: %v", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			t.Fatalf("Failed to stat created file: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("Expected file size %d, got %d", len(content), info.Size())
		}
	})

	t.Run("Validate", func(t *testing.T) {
		ctx := context.Background()
		tfs := synthfs.NewTestFileSystem()

		t.Run("Valid operation", func(t *testing.T) {
			op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
			err := op.Validate(ctx, tfs)
			if err != nil {
				t.Errorf("Expected validation to pass, got error: %v", err)
			}
		})

		t.Run("Empty path", func(t *testing.T) {
			op := synthfs.NewSerializableCreateFile("", []byte("content"), 0644)
			err := op.Validate(ctx, tfs)
			if err == nil {
				t.Errorf("Expected validation to fail for empty path")
			}
		})

		t.Run("Invalid mode", func(t *testing.T) {
			op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 01000)
			err := op.Validate(ctx, tfs)
			if err == nil {
				t.Errorf("Expected validation to fail for invalid mode")
			}
		})
	})

	t.Run("Rollback", func(t *testing.T) {
		op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
		tfs := synthfs.NewTestFileSystem()
		ctx := context.Background()

		// Execute operation
		err := op.Execute(ctx, tfs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file exists
		_, err = tfs.Stat("test.txt")
		if err != nil {
			t.Fatalf("File should exist after execute: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, tfs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file is gone
		_, err = tfs.Stat("test.txt")
		if err == nil {
			t.Errorf("File should not exist after rollback")
		}
	})

	t.Run("Conflicts", func(t *testing.T) {
		op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
		conflicts := op.Conflicts()

		// Should return nil (no conflicts implemented yet)
		if conflicts != nil {
			t.Errorf("Expected nil conflicts, got %v", conflicts)
		}
	})
}

func TestSerializableCreateFileOperation_JSON(t *testing.T) {
	t.Run("MarshalJSON", func(t *testing.T) {
		op := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
		op.WithID("test-id").WithDependency("dep-id")

		data, err := op.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		// Verify it's valid JSON
		var jsonData map[string]interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			t.Fatalf("Generated JSON is invalid: %v", err)
		}

		// Check required fields
		if jsonData["type"] != "create_file" {
			t.Errorf("Expected type 'create_file', got %v", jsonData["type"])
		}

		if jsonData["id"] != "test-id" {
			t.Errorf("Expected id 'test-id', got %v", jsonData["id"])
		}

		// Check dependencies
		deps, ok := jsonData["dependencies"].([]interface{})
		if !ok || len(deps) != 1 || deps[0] != "dep-id" {
			t.Errorf("Expected dependencies ['dep-id'], got %v", jsonData["dependencies"])
		}

		// Check parameters
		params, ok := jsonData["parameters"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected parameters object, got %v", jsonData["parameters"])
		}

		if params["path"] != "test.txt" {
			t.Errorf("Expected path 'test.txt', got %v", params["path"])
		}

		if params["content"] != "content" {
			t.Errorf("Expected content 'content', got %v", params["content"])
		}

		if params["mode"] != "644" {
			t.Errorf("Expected mode '644', got %v", params["mode"])
		}
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		jsonData := `{
			"type": "create_file",
			"id": "test-id",
			"dependencies": ["dep-id"],
			"parameters": {
				"path": "test.txt",
				"content": "content",
				"mode": "644"
			}
		}`

		var op synthfs.SerializableCreateFileOperation
		err := op.UnmarshalJSON([]byte(jsonData))
		if err != nil {
			t.Fatalf("UnmarshalJSON failed: %v", err)
		}

		if op.ID() != "test-id" {
			t.Errorf("Expected ID 'test-id', got %q", op.ID())
		}

		deps := op.Dependencies()
		if len(deps) != 1 || deps[0] != "dep-id" {
			t.Errorf("Expected dependencies ['dep-id'], got %v", deps)
		}

		desc := op.Describe()
		if desc.Path != "test.txt" {
			t.Errorf("Expected path 'test.txt', got %q", desc.Path)
		}
	})

	t.Run("UnmarshalJSON errors", func(t *testing.T) {
		testCases := []struct {
			name     string
			jsonData string
		}{
			{
				name:     "invalid JSON",
				jsonData: `{invalid json}`,
			},
			{
				name:     "wrong type",
				jsonData: `{"type": "wrong_type", "id": "test", "parameters": {}}`,
			},
			{
				name:     "missing path",
				jsonData: `{"type": "create_file", "id": "test", "parameters": {"content": "test", "mode": "644"}}`,
			},
			{
				name:     "missing content",
				jsonData: `{"type": "create_file", "id": "test", "parameters": {"path": "test.txt", "mode": "644"}}`,
			},
			{
				name:     "missing mode",
				jsonData: `{"type": "create_file", "id": "test", "parameters": {"path": "test.txt", "content": "test"}}`,
			},
			{
				name:     "invalid mode format",
				jsonData: `{"type": "create_file", "id": "test", "parameters": {"path": "test.txt", "content": "test", "mode": "invalid"}}`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var op synthfs.SerializableCreateFileOperation
				err := op.UnmarshalJSON([]byte(tc.jsonData))
				if err == nil {
					t.Errorf("Expected UnmarshalJSON to fail for %s", tc.name)
				}
			})
		}
	})

	t.Run("Round trip JSON", func(t *testing.T) {
		original := synthfs.NewSerializableCreateFile("test.txt", []byte("content"), 0644)
		original.WithID("test-id").WithDependency("dep-id")

		// Marshal
		data, err := original.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed: %v", err)
		}

		// Unmarshal
		var unmarshaled synthfs.SerializableCreateFileOperation
		err = unmarshaled.UnmarshalJSON(data)
		if err != nil {
			t.Fatalf("UnmarshalJSON failed: %v", err)
		}

		// Compare
		if unmarshaled.ID() != original.ID() {
			t.Errorf("ID mismatch: expected %q, got %q", original.ID(), unmarshaled.ID())
		}

		originalDeps := original.Dependencies()
		unmarshaledDeps := unmarshaled.Dependencies()
		if len(originalDeps) != len(unmarshaledDeps) {
			t.Errorf("Dependencies length mismatch: expected %d, got %d", len(originalDeps), len(unmarshaledDeps))
		}

		originalDesc := original.Describe()
		unmarshaledDesc := unmarshaled.Describe()
		if originalDesc.Path != unmarshaledDesc.Path {
			t.Errorf("Path mismatch: expected %q, got %q", originalDesc.Path, unmarshaledDesc.Path)
		}
	})
}
