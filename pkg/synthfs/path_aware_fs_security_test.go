package synthfs

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// TestPathAwareFS_SecurityEdgeCases tests critical security scenarios
func TestPathAwareFS_SecurityEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		base          string
		mode          PathMode
		path          string
		expectError   bool
		errorContains string
		setupFiles    map[string]string // files to create in test filesystem
	}{
		// Path traversal attack vectors
		{
			name:          "path traversal with ../",
			base:          "/safe",
			mode:          PathModeAuto,
			path:          "../../../etc/passwd",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		{
			name:          "path traversal in absolute mode",
			base:          "/safe",
			mode:          PathModeAbsolute,  
			path:          "/etc/passwd",
			expectError:   true,
			errorContains: "escapes filesystem root in Absolute mode",
		},
		{
			name:          "path traversal in relative mode",
			base:          "/safe",
			mode:          PathModeRelative,
			path:          "../../../etc/passwd",
			expectError:   true,
			errorContains: "escapes filesystem root in Relative mode",
		},
		{
			name:          "complex path traversal",
			base:          "/safe/project",
			mode:          PathModeAuto,
			path:          "../../sensitive/data",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		{
			name:          "nested directory traversal",
			base:          "/tmp/project",
			mode:          PathModeAuto,
			path:          "dir1/../../sensitive",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		// Edge cases that should be safe
		{
			name:        "legitimate relative path",
			base:        "/safe",
			mode:        PathModeAuto,
			path:        "subfolder/file.txt",
			expectError: false,
		},
		{
			name:        "root path access",
			base:        "/safe",
			mode:        PathModeAuto,
			path:        "/safe/file.txt",
			expectError: false,
		},
		{
			name:        "current directory reference",
			base:        "/safe",
			mode:        PathModeAuto,
			path:        "./file.txt",
			expectError: false,
		},
		// Boundary conditions
		{
			name:          "empty path",
			base:          "/safe",
			mode:          PathModeAuto,
			path:          "",
			expectError:   true,
			errorContains: "empty path",
		},
		{
			name:        "single dot path",
			base:        "/safe",
			mode:        PathModeAuto,
			path:        ".",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test filesystem
			testFS := filesystem.NewTestFileSystem()
			pfs := NewPathAwareFileSystem(testFS, tt.base).WithPathMode(tt.mode)

			// Setup test files if specified
			for path, content := range tt.setupFiles {
				if err := testFS.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to setup test file %s: %v", path, err)
				}
			}

			// Test path resolution through Open operation
			_, err := pfs.Open(tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					// File not found is OK for security test - we're testing path resolution
					var pathErr *fs.PathError
					if !isPathError(err, &pathErr) || pathErr.Err.Error() != "file does not exist" {
						t.Errorf("unexpected error: %v", err)
					}
				}
			}
		})
	}
}

// TestPathAwareFS_SymlinkSecurity tests symlink-related security scenarios
func TestPathAwareFS_SymlinkSecurity(t *testing.T) {
	tests := []struct {
		name          string
		base          string
		symlinkPath   string
		symlinkTarget string
		accessPath    string
		expectError   bool
		errorContains string
	}{
		{
			name:          "symlink pointing outside root",
			base:          "/safe",
			symlinkPath:   "/safe/link",
			symlinkTarget: "/etc/passwd",
			accessPath:    "link",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		{
			name:          "symlink with relative traversal",
			base:          "/safe",
			symlinkPath:   "/safe/link",
			symlinkTarget: "../../../etc/passwd",
			accessPath:    "link",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		{
			name:        "safe internal symlink",
			base:        "/safe",
			symlinkPath: "/safe/link",
			symlinkTarget: "internal/file.txt",
			accessPath:  "link",
			expectError: false,
		},
		// Note: This test case is removed because it's not applicable with real filesystem.
		// When using a real filesystem with a temp directory as root, absolute paths
		// like "/safe/internal/file.txt" don't make sense. The centralized symlink
		// resolution in PathHandler.ResolveSymlinkTarget() already handles the security
		// of absolute paths within the filesystem root, which is tested separately.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("SynthFS does not officially support Windows")
			}
			
			// Use real filesystem for security testing
			tempDir := t.TempDir()
			
			// Create the PathAwareFileSystem rooted at temp directory
			osFS := filesystem.NewOSFileSystem(tempDir)
			pfs := NewPathAwareFileSystem(osFS, tempDir)
			
			// For symlink tests, we need to test the security at symlink creation time
			// because that's where the real filesystem enforces security
			if tt.expectError {
				// These should fail at Symlink creation with real filesystem
				// Adjust the paths - remove /safe prefix since we're at root
				linkPath := strings.TrimPrefix(tt.symlinkPath, "/safe/")
				if linkPath == tt.symlinkPath {
					linkPath = strings.TrimPrefix(linkPath, "/")
				}
				
				err := pfs.Symlink(tt.symlinkTarget, linkPath)
				if err == nil {
					t.Error("Expected error creating dangerous symlink, but got none")
				} else {
					t.Logf("Got security error as expected: %v", err)
				}
			} else {
				// For safe symlinks, create the target first
				targetPath := strings.TrimPrefix(tt.symlinkTarget, "/safe/")
				if targetPath == tt.symlinkTarget {
					targetPath = strings.TrimPrefix(targetPath, "/")
				}
				
				// Skip if it's an absolute path within root - not testable with real FS
				if strings.HasPrefix(tt.symlinkTarget, "/safe/") {
					t.Skip("Absolute path symlinks within root not testable with real filesystem")
					return
				}
				
				// Create target file/directory
				if strings.Contains(targetPath, "/") {
					dir := filepath.Dir(targetPath)
					if err := pfs.MkdirAll(dir, 0755); err != nil {
						t.Fatalf("Failed to create target directory: %v", err)
					}
				}
				if err := pfs.WriteFile(targetPath, []byte("test content"), 0644); err != nil {
					t.Fatalf("Failed to create target file: %v", err)
				}
				
				// Create the symlink
				linkPath := strings.TrimPrefix(tt.symlinkPath, "/safe/")
				if linkPath == tt.symlinkPath {
					linkPath = strings.TrimPrefix(linkPath, "/")
				}
				err := pfs.Symlink(targetPath, linkPath)
				if err != nil {
					// Real filesystem may still reject some symlinks
					if strings.Contains(err.Error(), "invalid argument") {
						t.Skip("Filesystem doesn't support this type of symlink")
						return
					}
					t.Errorf("Failed to create safe symlink: %v", err)
				} else {
					// Verify we can read through the symlink
					_, err := pfs.Readlink(linkPath)
					if err != nil {
						t.Errorf("Failed to read symlink: %v", err)
					}
				}
			}
		})
	}
}

// TestPathAwareFS_GetPathHandlerSecurity tests direct path handler access
func TestPathAwareFS_GetPathHandlerSecurity(t *testing.T) {
	testFS := filesystem.NewTestFileSystem()
	pfs := NewPathAwareFileSystem(testFS, "/safe")

	// Test that GetPathHandler returns a working handler
	handler := pfs.GetPathHandler()
	if handler == nil {
		t.Fatal("GetPathHandler returned nil")
	}

	// Test that handler enforces same security rules
	_, err := handler.ResolvePath("../../../etc/passwd")
	if err == nil {
		t.Error("GetPathHandler should reject path traversal attacks")
	}

	// Test that handler allows safe paths
	resolved, err := handler.ResolvePath("safe/file.txt")
	if err != nil {
		t.Errorf("GetPathHandler should allow safe paths: %v", err)
	}
	
	expected := filepath.Join("/safe", "safe/file.txt")
	if resolved != expected {
		t.Errorf("expected %q but got %q", expected, resolved)
	}
}

// TestPathAwareFS_FallbackLogicSecurity tests the manual path resolution fallback
func TestPathAwareFS_FallbackLogicSecurity(t *testing.T) {
	testFS := filesystem.NewTestFileSystem()
	pfs := NewPathAwareFileSystem(testFS, "/safe")

	// Test scenarios that might trigger fallback logic in resolvePath
	tests := []struct {
		name          string
		path          string
		expectError   bool
		errorContains string
	}{
		{
			name:          "path with multiple slashes",
			path:          "//../../etc/passwd",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		{
			name:          "path with mixed separators",
			path:          "dir/../../../etc/passwd",
			expectError:   true,
			errorContains: "escapes filesystem root",
		},
		{
			name:        "complex but safe path",
			path:        "dir1/dir2/../dir3/file.txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pfs.Open(tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errorContains)
				}
			} else {
				// File not found is acceptable - we're testing path resolution security
				if err != nil {
					var pathErr *fs.PathError
					if !isPathError(err, &pathErr) || pathErr.Err.Error() != "file does not exist" {
						t.Errorf("unexpected error: %v", err)
					}
				}
			}
		})
	}
}

// TestPathAwareFS_InterfaceComplianceSecurity tests type assertion failure scenarios
func TestPathAwareFS_InterfaceComplianceSecurity(t *testing.T) {
	// Create a minimal filesystem that doesn't implement all interfaces
	minimalFS := &MinimalFileSystem{}
	pfs := NewPathAwareFileSystem(minimalFS, "/safe")

	// Test Stat fallback when StatFS is not implemented
	_, err := pfs.Stat("test.txt")
	if err == nil {
		t.Error("expected error from minimal filesystem")
	}

	// Verify the error is handled gracefully and doesn't expose internal details
	if containsString(err.Error(), "panic") || containsString(err.Error(), "assertion") {
		t.Errorf("error should not expose internal implementation details: %v", err)
	}
}

// TestPathAwareFS_EdgeCaseChaining tests combinations of path modes and edge cases
func TestPathAwareFS_EdgeCaseChaining(t *testing.T) {
	testFS := filesystem.NewTestFileSystem()
	
	// Test mode switching with same instance
	pfs := NewPathAwareFileSystem(testFS, "/safe")
	
	// Switch modes and test security is maintained
	modes := []struct {
		name string
		mode PathMode
	}{
		{"auto", PathModeAuto},
		{"absolute", PathModeAbsolute},
		{"relative", PathModeRelative},
	}

	for _, mode := range modes {
		t.Run("mode_"+mode.name, func(t *testing.T) {
			pfs = pfs.WithPathMode(mode.mode)
			
			// Test that security is maintained across mode switches
			_, err := pfs.Open("../../../etc/passwd")
			if err == nil {
				t.Errorf("mode %s should reject path traversal", mode.name)
			}
		})
	}
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (len(substr) == 0 || 
		    func() bool {
		        for i := 0; i <= len(s)-len(substr); i++ {
		            if s[i:i+len(substr)] == substr {
		                return true
		            }
		        }
		        return false
		    }())
}

func isPathError(err error, pathErr **fs.PathError) bool {
	if pe, ok := err.(*fs.PathError); ok {
		*pathErr = pe
		return true
	}
	return false
}

// MinimalFileSystem implements only the basic filesystem interfaces
type MinimalFileSystem struct{}

func (mfs *MinimalFileSystem) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (mfs *MinimalFileSystem) ReadFile(name string) ([]byte, error) {
	return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
}

func (mfs *MinimalFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrPermission}
}

func (mfs *MinimalFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return &fs.PathError{Op: "mkdir", Path: path, Err: fs.ErrPermission}
}

func (mfs *MinimalFileSystem) Remove(name string) error {
	return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrPermission}
}

func (mfs *MinimalFileSystem) Rename(oldname, newname string) error {
	return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrPermission}
}

func (mfs *MinimalFileSystem) Symlink(oldname, newname string) error {
	return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrPermission}
}

func (mfs *MinimalFileSystem) Readlink(name string) (string, error) {
	return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
}

func (mfs *MinimalFileSystem) RemoveAll(path string) error {
	return &fs.PathError{Op: "removeall", Path: path, Err: fs.ErrPermission}
}

func (mfs *MinimalFileSystem) Stat(name string) (fs.FileInfo, error) {
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}