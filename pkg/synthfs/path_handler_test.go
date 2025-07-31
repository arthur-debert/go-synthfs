package synthfs

import (
	"strings"
	"testing"
)

func TestPathHandler(t *testing.T) {
	t.Run("NewPathHandler", func(t *testing.T) {
		tests := []struct {
			name     string
			base     string
			wantBase string
		}{
			{"empty base", "", "/"},
			{"relative base", "relative/path", "relative/path"}, // May become absolute
			{"absolute base", "/absolute/path", "/absolute/path"},
			{"base with ..", "/path/../other", "/other"},
			{"base with double slash", "/path//to///dir", "/path/to/dir"},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ph := NewPathHandler(tt.base, PathModeAuto)
				if tt.wantBase == "relative/path" {
					// For relative paths, just check it's not empty
					if ph.base == "" {
						t.Error("Base should not be empty")
					}
				} else if ph.base != tt.wantBase && !strings.HasSuffix(ph.base, tt.wantBase) {
					t.Errorf("Expected base %q, got %q", tt.wantBase, ph.base)
				}
			})
		}
	})
	
	t.Run("Auto mode", func(t *testing.T) {
		ph := NewPathHandler("/base", PathModeAuto)
		
		tests := []struct {
			name    string
			path    string
			want    string
			wantErr bool
		}{
			{"relative path", "sub/dir", "/base/sub/dir", false},
			{"relative with ./", "./sub/dir", "/base/sub/dir", false},
			{"absolute within base", "/base/sub/dir", "/base/sub/dir", false},
			{"absolute outside base", "/other/path", "", true},
			{"path with ..", "sub/../other", "/base/other", false},
			{"escaping path", "../../../etc", "", true},
			{"empty path", "", "", true},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ph.ResolvePath(tt.path)
				if (err != nil) != tt.wantErr {
					t.Errorf("ResolvePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("ResolvePath(%q) = %q, want %q", tt.path, got, tt.want)
				}
			})
		}
	})
	
	t.Run("Absolute mode", func(t *testing.T) {
		ph := NewPathHandler("/base", PathModeAbsolute)
		
		tests := []struct {
			name    string
			path    string
			want    string
			wantErr bool
		}{
			{"absolute path", "/base/file", "/base/file", false},
			{"relative treated as absolute", "file", "/file", true}, // Outside base
			{"relative with base prefix", "base/file", "/base/file", false},
			{"path outside base", "/other/file", "", true},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ph.ResolvePath(tt.path)
				if (err != nil) != tt.wantErr {
					t.Errorf("ResolvePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ResolvePath(%q) = %q, want %q", tt.path, got, tt.want)
				}
			})
		}
	})
	
	t.Run("Relative mode", func(t *testing.T) {
		ph := NewPathHandler("/base", PathModeRelative)
		
		tests := []struct {
			name    string
			path    string
			want    string
			wantErr bool
		}{
			{"relative path", "sub/file", "/base/sub/file", false},
			{"absolute treated as relative", "/sub/file", "/base/sub/file", false},
			{"escaping path", "../file", "", true},
			{"clean path", "./sub/../other/file", "/base/other/file", false},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ph.ResolvePath(tt.path)
				if (err != nil) != tt.wantErr {
					t.Errorf("ResolvePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ResolvePath(%q) = %q, want %q", tt.path, got, tt.want)
				}
			})
		}
	})
	
	t.Run("MakeRelative", func(t *testing.T) {
		ph := NewPathHandler("/base/dir", PathModeAuto)
		
		tests := []struct {
			name    string
			path    string
			want    string
			wantErr bool
		}{
			{"already relative", "sub/file", "sub/file", false},
			{"absolute within base", "/base/dir/sub/file", "sub/file", false},
			{"absolute at base", "/base/dir", ".", false},
			{"absolute outside base", "/other/path", "", true},
			{"parent of base", "/base", "", true},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ph.MakeRelative(tt.path)
				if (err != nil) != tt.wantErr {
					t.Errorf("MakeRelative(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("MakeRelative(%q) = %q, want %q", tt.path, got, tt.want)
				}
			})
		}
	})
	
	t.Run("NormalizePath", func(t *testing.T) {
		tests := []struct {
			path string
			want string
		}{
			{"path//with///slashes", "path/with/slashes"},
			{"./path/../other", "other"},
			{"/absolute/../path", "/path"},
			{"", "."},
			{".", "."},
			{"..", ".."},
		}
		
		for _, tt := range tests {
			t.Run(tt.path, func(t *testing.T) {
				got := NormalizePath(tt.path)
				if got != tt.want {
					t.Errorf("NormalizePath(%q) = %q, want %q", tt.path, got, tt.want)
				}
			})
		}
	})
}

func TestPathAwareFileSystem(t *testing.T) {
	t.Run("Path resolution", func(t *testing.T) {
		fs := NewTestFileSystemWithPaths("/project")
		
		// Write a file using different path styles
		content := []byte("test content")
		
		// Relative path
		err := fs.WriteFile("data/file1.txt", content, 0644)
		if err != nil {
			t.Errorf("Failed to write with relative path: %v", err)
		}
		
		// Absolute path within base
		err = fs.WriteFile("/project/data/file2.txt", content, 0644)
		if err != nil {
			t.Errorf("Failed to write with absolute path: %v", err)
		}
		
		// Path with ./
		err = fs.WriteFile("./data/file3.txt", content, 0644)
		if err != nil {
			t.Errorf("Failed to write with ./ path: %v", err)
		}
		
		// Verify all files exist
		for _, path := range []string{"data/file1.txt", "/project/data/file2.txt", "./data/file3.txt"} {
			if _, err := fs.Stat(path); err != nil {
				t.Errorf("Failed to stat %q: %v", path, err)
			}
		}
	})
	
	t.Run("Path modes", func(t *testing.T) {
		// Test absolute mode
		fs := NewTestFileSystemWithPaths("/workspace").WithAbsolutePaths()
		
		err := fs.WriteFile("/workspace/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Errorf("Failed to write in absolute mode: %v", err)
		}
		
		// This should fail - outside root
		err = fs.WriteFile("/other/file.txt", []byte("content"), 0644)
		if err == nil {
			t.Error("Expected error writing outside root in absolute mode")
		}
		
		// Test relative mode
		fs = NewTestFileSystemWithPaths("/workspace").WithRelativePaths()
		
		err = fs.WriteFile("relative/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Errorf("Failed to write in relative mode: %v", err)
		}
		
		// Absolute paths should be converted to relative
		err = fs.WriteFile("/absolute/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Errorf("Failed to write absolute as relative: %v", err)
		}
	})
	
	t.Run("Security - path traversal", func(t *testing.T) {
		fs := NewTestFileSystemWithPaths("/safe/dir")
		
		// These should all fail
		badPaths := []string{
			"../../../etc/passwd",
			"/etc/passwd",
			"data/../../../../../../etc/passwd",
			"/safe/../../../etc/passwd",
		}
		
		for _, path := range badPaths {
			err := fs.WriteFile(path, []byte("evil"), 0644)
			if err == nil {
				t.Errorf("Expected error for malicious path %q", path)
			}
		}
	})
	
	t.Run("All operations", func(t *testing.T) {
		fs := NewTestFileSystemWithPaths("/app")
		
		// Create directory
		err := fs.MkdirAll("config", 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		
		// Write file
		err = fs.WriteFile("config/app.json", []byte("{}"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		
		// Rename
		err = fs.Rename("config/app.json", "config/app.yaml")
		if err != nil {
			t.Fatalf("Rename failed: %v", err)
		}
		
		// Create symlink
		err = fs.Symlink("config/app.yaml", "app.yaml")
		if err != nil {
			t.Fatalf("Symlink failed: %v", err)
		}
		
		// Read symlink
		target, err := fs.Readlink("app.yaml")
		if err != nil {
			t.Fatalf("Readlink failed: %v", err)
		}
		if target != "config/app.yaml" {
			t.Errorf("Expected symlink target 'config/app.yaml', got %q", target)
		}
		
		// Remove file
		err = fs.Remove("config/app.yaml")
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}
		
		// Remove all
		err = fs.RemoveAll("config")
		if err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	})
}