package synthfs

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathHandler_ResolveSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	tests := []struct {
		name       string
		base       string
		linkPath   string
		targetPath string
		want       string
		wantErr    bool
		errMsg     string
	}{
		// Basic cases
		{
			name:       "relative target in same directory",
			base:       "/workspace",
			linkPath:   "/workspace/link",
			targetPath: "file.txt",
			want:       "/workspace/file.txt",
			wantErr:    false,
		},
		{
			name:       "relative target in subdirectory",
			base:       "/workspace",
			linkPath:   "/workspace/link",
			targetPath: "subdir/file.txt",
			want:       "/workspace/subdir/file.txt",
			wantErr:    false,
		},
		{
			name:       "relative target with parent directory",
			base:       "/workspace",
			linkPath:   "/workspace/subdir/link",
			targetPath: "../file.txt",
			want:       "/workspace/file.txt",
			wantErr:    false,
		},
		{
			name:       "absolute target within root",
			base:       "/workspace",
			linkPath:   "/workspace/link",
			targetPath: "/workspace/file.txt",
			want:       "/workspace/file.txt",
			wantErr:    false,
		},

		// Security cases - should fail
		{
			name:       "relative target escaping root",
			base:       "/workspace",
			linkPath:   "/workspace/link",
			targetPath: "../../../etc/passwd",
			want:       "",
			wantErr:    true,
			errMsg:     "escapes filesystem root",
		},
		{
			name:       "absolute target outside root",
			base:       "/workspace",
			linkPath:   "/workspace/link",
			targetPath: "/etc/passwd",
			want:       "",
			wantErr:    true,
			errMsg:     "escapes filesystem root",
		},
		{
			name:       "relative target from subdirectory escaping root",
			base:       "/workspace",
			linkPath:   "/workspace/subdir/link",
			targetPath: "../../outside",
			want:       "",
			wantErr:    true,
			errMsg:     "escapes filesystem root",
		},

		// Complex cases
		{
			name:       "deeply nested relative target",
			base:       "/workspace",
			linkPath:   "/workspace/a/b/c/link",
			targetPath: "../../../file.txt",
			want:       "/workspace/file.txt",
			wantErr:    false,
		},
		{
			name:       "relative link path with relative target",
			base:       "/workspace",
			linkPath:   "subdir/link",
			targetPath: "../file.txt",
			want:       "/workspace/file.txt",
			wantErr:    false,
		},
		{
			name:       "root filesystem allows any absolute path",
			base:       "/",
			linkPath:   "/home/user/link",
			targetPath: "/etc/passwd",
			want:       "/etc/passwd",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ph := NewPathHandler(tt.base, PathModeAuto)
			got, err := ph.ResolveSymlinkTarget(tt.linkPath, tt.targetPath)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveSymlinkTarget() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ResolveSymlinkTarget() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}
			
			if err != nil {
				t.Errorf("ResolveSymlinkTarget() unexpected error = %v", err)
				return
			}
			
			// Normalize paths for comparison
			got = filepath.Clean(got)
			want := filepath.Clean(tt.want)
			
			if got != want {
				t.Errorf("ResolveSymlinkTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test specific mirror operation scenarios
func TestPathHandler_ResolveSymlinkTarget_MirrorScenarios(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	// This tests the specific cases that were failing in mirror operations
	tests := []struct {
		name       string
		base       string
		linkPath   string
		targetPath string
		want       string
		wantErr    bool
	}{
		{
			name:       "mirror operation - bin symlink",
			base:       "/tmp/test",
			linkPath:   "/tmp/test/mirror/bin",
			targetPath: "../source/bin",
			want:       "/tmp/test/source/bin",
			wantErr:    false,
		},
		{
			name:       "mirror operation - nested file symlink",
			base:       "/tmp/test",
			linkPath:   "/tmp/test/configs/config/db.yaml",
			targetPath: "../../data/config/db.yaml",
			want:       "/tmp/test/data/config/db.yaml",
			wantErr:    false,
		},
		{
			name:       "mirror operation - file in subdirectory",
			base:       "/tmp/test",
			linkPath:   "/tmp/test/build/test/app_test.go",
			targetPath: "../../src/test/app_test.go",
			want:       "/tmp/test/src/test/app_test.go",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ph := NewPathHandler(tt.base, PathModeAuto)
			got, err := ph.ResolveSymlinkTarget(tt.linkPath, tt.targetPath)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveSymlinkTarget() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			
			if err != nil {
				t.Errorf("ResolveSymlinkTarget() unexpected error = %v", err)
				return
			}
			
			// Normalize paths for comparison
			got = filepath.Clean(got)
			want := filepath.Clean(tt.want)
			
			if got != want {
				t.Errorf("ResolveSymlinkTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}