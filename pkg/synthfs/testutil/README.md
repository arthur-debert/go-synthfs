# SynthFS Test Utilities

This package provides testing utilities for SynthFS. 

## Real Filesystem Testing (Recommended)

**IMPORTANT**: All new tests should use `RealFSTestHelper` for real filesystem testing.

### Quick Start

```go
func TestMyFeature(t *testing.T) {
    helper := testutil.NewRealFSTestHelper(t)
    fs := helper.FS
    
    // Write your test using real filesystem
    err := fs.WriteFile("test.txt", []byte("content"), 0644)
    // ...
}
```

The helper automatically:
- Skips tests on Windows (SynthFS is Unix-only)
- Creates an isolated temporary directory
- Provides utilities for symlink testing
- Cleans up after the test

### Why Real Filesystem?

The migration from mock filesystem to real filesystem testing revealed:
- **Security vulnerabilities**: Dangerous symlink patterns that mocks allowed
- **Behavioral differences**: Real filesystems have constraints mocks don't enforce
- **Missing validations**: Permission checks, path restrictions, etc.

See [docs/dev/README.md](../../../docs/dev/README.md) for comprehensive testing guidelines.

## Available Utilities

### RealFSTestHelper (realfs.go)
Primary test helper for real filesystem testing. Use this for all new tests.

### Mock Utilities (Legacy)
- `mock_fs.go`: Mock filesystem implementation (deprecated, use real filesystem)
- `operations_mock.go`: Mock operations for testing
- `testing.go`: General testing utilities

## Examples

See the migrated tests throughout the codebase for examples:
- `patterns_*_test.go`: Pattern tests using real filesystem
- `path_*_test.go`: Path handling tests with real filesystem

All tests demonstrate proper real filesystem usage and handling of platform-specific behaviors.