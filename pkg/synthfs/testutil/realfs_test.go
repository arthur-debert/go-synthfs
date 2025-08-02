package testutil

import (
	"runtime"
	"testing"
)

func TestRealFSTestHelper_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}
	
	helper := NewRealFSTestHelper(t)
	
	// Test basic filesystem access
	fs := helper.FileSystem()
	if fs == nil {
		t.Fatal("FileSystem() returned nil")
	}
	
	// Test temp directory access
	tempDir := helper.TempDir()
	if tempDir == "" {
		t.Fatal("TempDir() returned empty string")
	}
	
	// Test file creation
	CreateTestFile(t, fs, "test.txt", []byte("hello"))
	AssertFileContent(t, fs, "test.txt", []byte("hello"))
}

func TestRealFSTestHelper_Symlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}
	
	helper := NewRealFSTestHelper(t)
	fs := helper.FileSystem()
	
	// Create a target file
	CreateTestFile(t, fs, "target.txt", []byte("target content"))
	
	// Create a symlink to it
	helper.CreateSymlink("target.txt", "link.txt")
	
	// Verify symlink exists and points to correct target
	helper.AssertSymlinkExists("link.txt")
	helper.AssertSymlinkTarget("link.txt", "target.txt")
	
	// Verify we can read through the symlink
	AssertFileContent(t, fs, "link.txt", []byte("target content"))
}

func TestRealFSTestHelper_SymlinkSecurity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}
	
	helper := NewRealFSTestHelper(t)
	
	// Test basic symlink operations - the security testing will be done
	// at the PathAwareFS level, not at the raw filesystem level
	helper.CreateSymlink("target.txt", "normal_link")
	helper.AssertSymlinkTarget("normal_link", "target.txt")
	
	// Test directory symlinks
	CreateTestDir(t, helper.FileSystem(), "test_dir")
	helper.CreateSymlink("test_dir", "dir_link")
	helper.AssertSymlinkTarget("dir_link", "test_dir")
}