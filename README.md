# SynthFS - Simplified Filesystem Operations

A Go library for simplified filesystem operations with automatic dependency resolution and batch execution.

## 🚀 **Key Features**

- **🔄 Automatic Dependencies**: Parent directories and conflict resolution handled automatically
- **📦 Batch Operations**: Execute multiple filesystem operations as a unit with rollback support
- **🧪 Clean Architecture**: Single implementation with prerequisite resolution built-in
- **⚡ Performance**: Efficient operation execution with comprehensive error handling
- **🛡️ Safe Operations**: Built-in validation and conflict detection

## 🏗️ **Quick Start**

### Basic Usage

```go
package main

import (
    "github.com/arthur-debert/synthfs/pkg/synthfs"
    "github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
    "github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func main() {
    // Create filesystem and registry
    fs := filesystem.NewOSFileSystem(".")
    registry := operations.NewFactory()
    
    // Create batch with automatic prerequisite resolution
    batch := synthfs.NewBatch(fs, registry)
    
    // Add operations - parent directories handled automatically
    batch.CreateFile("deep/nested/structure/file.txt", []byte("content"))
    batch.CreateDir("another/directory")
    batch.Copy("source.txt", "backup/source.txt")
    
    // Execute all operations
    result, err := batch.Run()
    if err != nil {
        log.Fatal(err)
    }
    
    if result.IsSuccess() {
        fmt.Println("All operations completed successfully!")
    }
}
```

### Restorable Operations

```go
// Enable backup/restore functionality
result, err := batch.RunRestorable()
if err != nil {
    log.Printf("Operations failed: %v", err)
    
    // Use restore operations if needed
    if restoreOps := result.GetRestoreOps(); len(restoreOps) > 0 {
        // Execute restore operations...
    }
}
```

## � **Supported Operations**

| Operation | Description | Auto-resolves |
|-----------|-------------|---------------|
| `CreateFile()` | Create files with content | Parent directories |
| `CreateDir()` | Create directories | Parent directories |
| `Copy()` | Copy files/directories | Parent directories, source validation |
| `Move()` | Move files/directories | Parent directories, source validation |
| `Delete()` | Delete files/directories | Conflict checking |
| `CreateSymlink()` | Create symbolic links | Parent directories |
| `CreateArchive()` | Create .tar.gz/.zip archives | Parent directories |
| `Unarchive()` | Extract archives | Parent directories |

## ✨ **Architecture**

### Clean Design

```go
// Single constructor - no feature flags or complex options
batch := synthfs.NewBatch(fs, registry)

// Operations declare prerequisites automatically
batch.CreateFile("path/to/file.txt", content)

// Prerequisite resolution happens during execution
result, err := batch.Run()
```

### Automatic Dependency Management

The library automatically handles:

- **Parent Directory Creation**: `CreateFile("a/b/c/file.txt")` creates `a/`, `a/b/`, and `a/b/c/` as needed
- **Conflict Detection**: Prevents overwriting existing files unless explicitly intended
- **Source Validation**: Ensures source files exist before copy/move operations
- **Dependency Ordering**: Operations execute in the correct order based on dependencies

### Error Handling

```go
result, err := batch.Run()
if err != nil {
    // Handle execution errors
    fmt.Printf("Batch failed: %v\n", err)
}

if !result.IsSuccess() {
    // Handle partial failures
    if execErr := result.GetError(); execErr != nil {
        fmt.Printf("First error: %v\n", execErr)
    }
}
```

## � **Development**

### Building

```bash
# Build all packages
./scripts/build

# Run tests with coverage
./scripts/test-with-coverage

# Run linting
./scripts/lint
```

### Project Structure

- **`pkg/synthfs/`** - Main library packages
  - **`batch/`** - Batch operation implementation
  - **`core/`** - Core interfaces and types  
  - **`operations/`** - Individual operation implementations
  - **`execution/`** - Execution pipeline and prerequisite resolution
  - **`targets/`** - Target item types (files, directories, etc.)
  - **`filesystem/`** - Filesystem abstraction layer

### Testing

```bash
# Run all tests
./scripts/test

# Generate coverage report
./scripts/test-with-coverage
open coverage.html
```

## 📚 **Advanced Usage**

### Custom Filesystem

```go
// Use custom filesystem implementation
fs := &MyCustomFileSystem{}
batch := synthfs.NewBatch(fs, registry)
```

### Logging

```go
logger := &MyLogger{} // Implement core.Logger interface
batch := synthfs.NewBatch(fs, registry).WithLogger(logger)
```

### Context Support

```go
ctx := context.WithTimeout(context.Background(), 30*time.Second)
batch := synthfs.NewBatch(fs, registry).WithContext(ctx)
```

## 📊 **Performance**

- **Efficient Resolution**: Prerequisites resolved once during planning phase
- **Batch Execution**: Operations executed in optimal order
- **Memory Efficient**: Minimal memory footprint for operation tracking
- **Concurrent Safe**: Thread-safe operation building (execution is single-threaded)

## �️ **Safety Features**

- **Validation**: All operations validated before execution
- **Rollback**: Optional backup/restore functionality
- **Conflict Detection**: Prevents accidental overwrites
- **Error Recovery**: Comprehensive error reporting with context

## 📄 **License**

MIT License - see LICENSE file for details

## 🤝 **Contributing**

1. Fork the repository
2. Create a feature branch
3. Run tests: `./scripts/test`
4. Submit a pull request

See the development scripts in `scripts/` directory for building, testing, and linting.
