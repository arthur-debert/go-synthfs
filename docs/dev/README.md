# Development Guidelines

## Testing Best Practices

### Use Real Filesystem for Tests

**IMPORTANT**: All new tests should use real filesystem testing instead of the deprecated `TestFileSystem`. The mock filesystem was found to hide critical security issues and behavioral differences from real filesystems.

### How to Write Tests with Real Filesystem

1. **Basic Setup**:
```go
import (
    "testing"
    "runtime"
    "github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestMyFeature(t *testing.T) {
    if runtime.GOOS == "windows" {
        t.Skip("SynthFS does not officially support Windows")
    }
    
    // Create temporary directory for test isolation
    tempDir := t.TempDir()
    osFS := filesystem.NewOSFileSystem(tempDir)
    fs := NewPathAwareFileSystem(osFS, tempDir)
    
    // Your test code here...
}
```

2. **Using the Test Helper** (recommended):
```go
import (
    "testing"
    "github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestMyFeature(t *testing.T) {
    helper := testutil.NewRealFSTestHelper(t)
    fs := helper.FS
    
    // The helper automatically:
    // - Skips on Windows
    // - Creates temp directory
    // - Provides helper methods for symlinks
    // - Cleans up after test
}
```

### Why Real Filesystem Testing?

During the migration from TestFileSystem to real filesystem testing, we discovered:

1. **Security Issues**: TestFileSystem allowed dangerous relative path symlinks (e.g., `../../../etc/passwd`) that real filesystems reject
2. **Behavioral Differences**: Operations like sync and mirror behave differently with real filesystem constraints
3. **Missing Validations**: Real filesystems enforce permissions, path restrictions, and other constraints that mocks missed
4. **Production Accuracy**: Tests now validate actual production behavior, not idealized mock behavior

### Common Patterns

**Creating Directories Before Writing Files**:
```go
// Real filesystem requires parent directories to exist
err := fs.MkdirAll("path/to", 0755)
if err != nil {
    t.Fatal(err)
}
err = fs.WriteFile("path/to/file.txt", []byte("content"), 0644)
```

**Handling Symlink Restrictions**:
```go
// Real filesystem may reject relative path symlinks
err := fs.Symlink("../target", "link")
if err != nil && strings.Contains(err.Error(), "invalid argument") {
    t.Skip("Relative symlinks not supported by filesystem")
}
```

**Path-Aware Testing**:
```go
// Use PathAwareFileSystem for proper path handling
fs := NewPathAwareFileSystem(osFS, tempDir)

// Test different path modes
fs.WithAbsolutePaths()  // Requires absolute paths
fs.WithRelativePaths()  // Converts all paths to relative
fs.WithAutoDetectPaths() // Auto-detects path type (default)
```

### Migration Status

All existing tests have been migrated from TestFileSystem to real filesystem. The migration revealed several bugs that have been documented as GitHub issues (#37, #38, #39, #40).

### Further Reading

- See `pkg/synthfs/testutil/realfs.go` for the test helper implementation
- See `CLAUDE.md` for general testing strategy
- Review migrated tests in `pkg/synthfs/*_test.go` for examples