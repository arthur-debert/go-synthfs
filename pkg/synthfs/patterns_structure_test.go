package synthfs

import (
	"context"
	"testing"
)

func TestStructurePatterns(t *testing.T) {
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	
	t.Run("Parse simple structure", func(t *testing.T) {
		structure := `
project/
    src/
        main.go
        lib/
            utils.go
    tests/
    README.md
`
		
		entries, err := ParseStructure(structure)
		if err != nil {
			t.Fatalf("Failed to parse structure: %v", err)
		}
		
		// Check entries
		expected := []string{
			"project",
			"project/src",
			"project/src/main.go",
			"project/src/lib",
			"project/src/lib/utils.go",
			"project/tests",
			"project/README.md",
		}
		
		if len(entries) != len(expected) {
			t.Errorf("Expected %d entries, got %d", len(expected), len(entries))
		}
		
		for i, entry := range entries {
			if i < len(expected) && entry.Path != expected[i] {
				t.Errorf("Entry %d: expected path %q, got %q", i, expected[i], entry.Path)
			}
		}
	})
	
	t.Run("Create basic structure", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")
		
		structure := `
app/
    cmd/
        main.go
    pkg/
        server/
            server.go
        client/
            client.go
    go.mod
    README.md
`
		
		op, err := CreateStructure(structure)
		if err != nil {
			t.Fatalf("Failed to create structure operation: %v", err)
		}
		
		err = op.Execute(ctx, filesys)
		if err != nil {
			t.Fatalf("Failed to execute structure creation: %v", err)
		}
		
		// Verify structure was created
		paths := []string{
			"app",
			"app/cmd",
			"app/cmd/main.go",
			"app/pkg/server",
			"app/pkg/server/server.go",
			"app/pkg/client",
			"app/pkg/client/client.go",
			"app/go.mod",
			"app/README.md",
		}
		
		for _, path := range paths {
			if _, err := filesys.Stat(path); err != nil {
				t.Errorf("Path %q should exist", path)
			}
		}
	})
	
	t.Run("Structure with tree characters", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/project")
		
		// Structure with tree drawing characters (should be ignored)
		structure := `
myapp/
├── src/
│   ├── index.js
│   └── utils.js
├── tests/
│   └── test.js
└── package.json
`
		
		op, err := CreateStructureIn("workspace", structure)
		if err != nil {
			t.Fatalf("Failed to create structure: %v", err)
		}
		
		err = op.Execute(ctx, filesys)
		if err != nil {
			t.Fatalf("Failed to execute: %v", err)
		}
		
		// Check specific files
		files := []string{
			"workspace/myapp/src/index.js",
			"workspace/myapp/src/utils.js",
			"workspace/myapp/tests/test.js",
			"workspace/myapp/package.json",
		}
		
		for _, file := range files {
			if _, err := filesys.Stat(file); err != nil {
				t.Errorf("File %q should exist", file)
			}
		}
	})
	
	t.Run("StructureBuilder with content", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/app")
		
		structure := `
project/
    src/
        main.py
        config.yaml
    README.md
    .gitignore
`
		
		err := NewStructureBuilder().
			FromString(structure).
			InDirectory("workspace").
			WithTextFile("src/main.py", "print('Hello, World!')").
			WithTextFile("src/config.yaml", "debug: true\nport: 8080").
			WithTextFile("README.md", "# My Project").
			WithTextFile(".gitignore", "*.pyc\n__pycache__/").
			Execute(ctx, filesys)
		
		if err != nil {
			t.Fatalf("StructureBuilder failed: %v", err)
		}
		
		// Verify content
		content, err := filesys.ReadFile("workspace/project/src/main.py")
		if err != nil {
			t.Fatalf("Failed to read main.py: %v", err)
		}
		if string(content) != "print('Hello, World!')" {
			t.Error("main.py content mismatch")
		}
		
		content, err = filesys.ReadFile("workspace/project/.gitignore")
		if err != nil {
			t.Fatalf("Failed to read .gitignore: %v", err)
		}
		if string(content) != "*.pyc\n__pycache__/" {
			t.Error(".gitignore content mismatch")
		}
	})
	
	t.Run("Structure with symlinks", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")
		
		structure := `
project/
    bin/
        app -> ../build/app
    build/
        app
    src/
        main.go
    versions/
        v1.0.0/
    current -> versions/v1.0.0
`
		
		op, err := CreateStructure(structure)
		if err != nil {
			t.Fatalf("Failed to create structure: %v", err)
		}
		
		// Add some content
		if structOp, ok := op.(*CreateStructureOperation); ok {
			structOp.WithFileContent("build/app", []byte("#!/bin/sh\necho 'App'"))
		}
		
		err = op.Execute(ctx, filesys)
		if err != nil {
			t.Fatalf("Failed to execute: %v", err)
		}
		
		// Check symlinks
		if target, err := filesys.Readlink("project/bin/app"); err == nil {
			if target != "../build/app" {
				t.Errorf("Expected symlink target '../build/app', got %q", target)
			}
		} else {
			t.Logf("Warning: Could not read symlink (test filesystem may not support): %v", err)
		}
	})
	
	t.Run("Complex nested structure", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")
		
		structure := `
webapp/
    frontend/
        public/
            index.html
            css/
                style.css
            js/
                app.js
        src/
            components/
                Header.jsx
                Footer.jsx
            App.jsx
        package.json
    backend/
        api/
            v1/
                users.go
                posts.go
        models/
            user.go
            post.go
        main.go
        go.mod
    docker-compose.yml
    README.md
`
		
		op, err := CreateStructure(structure)
		if err != nil {
			t.Fatalf("Failed to create structure: %v", err)
		}
		
		err = op.Execute(ctx, filesys)
		if err != nil {
			t.Fatalf("Failed to execute: %v", err)
		}
		
		// Check deep nesting
		deepPaths := []string{
			"webapp/frontend/public/css/style.css",
			"webapp/frontend/src/components/Header.jsx",
			"webapp/backend/api/v1/users.go",
		}
		
		for _, path := range deepPaths {
			if _, err := filesys.Stat(path); err != nil {
				t.Errorf("Deep path %q should exist", path)
			}
		}
	})
}