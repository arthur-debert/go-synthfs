# SynthFS - Lazy Filesystem Operations with Automatic Dependencies

A Go library that makes filesystem operations **testable**, **safe**, and **simple** through lazy evaluation, automatic dependency resolution, and optional rollback capabilities.

## Why SynthFS?

Filesystem operations in applications are notoriously difficult to:

- **Test reliably** - Hard to mock complex filesystem interactions and validate outcomes
- **Make safe** - Risk of data loss from partial failures with no easy recovery
- **Reason about** - Dependencies between operations are implicit and error-prone
- **Handle errors** - Cleanup after failures requires complex, brittle rollback logic

SynthFS solves these problems by providing **lazy operations** with automatic dependency resolution, multi-layer validation, and budget-controlled backup capabilities.

## 🚀 **Key Features**

- **🔄 Automatic Dependencies**: Parent directories and prerequisites resolved automatically
- **📦 Batch Operations**: Execute multiple operations atomically with optional rollback
- **🛡️ Multi-layer Validation**: Catch errors early with source validation and checksum verification
- **💾 Budget-controlled Backups**: Predictable memory usage for restoration (default 10MB)
- **🧪 Pure Functions**: Side-effect-free operation creation for easy testing
- **⚡ Smart Execution**: Topological sorting, conflict detection, and prerequisite resolution

## 🎯 **Use Cases**

- **Application Setup**: Initialize configuration directories and files safely
- **Development Tools**: Project scaffolding and template expansion with rollback
- **Deployment Scripts**: Reversible filesystem changes with automatic cleanup
- **Backup/Restore**: Reliable data migration with memory-controlled restoration
- **Testing**: Deterministic filesystem state management for integration tests

## 🏗️ **Quick Start**

### Basic Application Setup

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/arthur-debert/synthfs/pkg/synthfs"
    "github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
    "github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func main() {
    // Create filesystem and operation registry
    fs := filesystem.NewOSFileSystem(".")
    registry := operations.NewFactory()
    
    // Create batch - automatic dependency resolution enabled by default
    batch := synthfs.NewBatch(fs, registry)
    
    // Add operations (parent directories created automatically)
    _, err := batch.CreateDir("config")
    if err != nil {
        log.Fatal(err)
    }
    
    configData := []byte(`app_name: "myapp"\nversion: "1.0"`)
    _, err = batch.CreateFile("config/app.yaml", configData)
    if err != nil {
        log.Fatal(err)
    }
    
    // Create symlink (target validation is flexible)
    _, err = batch.CreateSymlink("config/app.yaml", "app.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Execute all operations with dependency resolution
    result, err := batch.Run()
    if err != nil {
        log.Fatalf("Batch execution failed: %v", err)
    }
    
    if result.IsSuccess() {
        fmt.Println("Application setup completed successfully!")
    }
}
```

### Restorable Operations with Backup

```go
// Enable backup/restore with custom budget
result, err := batch.RunRestorableWithBudget(50) // 50MB backup budget
if err != nil {
    log.Printf("Operations failed: %v", err)
    
    // Get restore operations if available
    if restoreOps := result.GetRestoreOps(); len(restoreOps) > 0 {
        fmt.Printf("Restoration available: %d operations", len(restoreOps))
        // Execute restore operations if needed
    }
}
```

## ✨ **Supported Operations**

| Operation | Description | Auto-resolves | Supports Backup |
|-----------|-------------|---------------|------------------|
| `CreateFile()` | Create files with content and permissions | Parent directories | ✅ |
| `CreateDir()` | Create directory hierarchies | Parent directories | ✅ |
| `Copy()` | Copy files/directories with metadata | Parent directories, source validation | ✅ |
| `Move()` | Move files/directories to new paths | Parent directories, source validation | ✅ |
| `Delete()` | Delete files/directories recursively | Conflict checking | ✅ |
| `CreateSymlink()` | Create symbolic links | Parent directories | ✅ |
| `CreateArchive()` | Create .tar.gz/.zip archives | Parent directories, source validation | ✅ |
| `Unarchive()` | Extract archives completely | Parent directories | ❌ |
| `UnarchiveWithPatterns()` | Extract archives selectively | Parent directories, pattern filtering | ❌ |

## 🏛️ **Architecture Overview**

SynthFS uses a three-layer architecture for maximum safety and flexibility:

- **Batch Layer**: Declarative API for "what to do" - collects operations with immediate validation
- **Pipeline Layer**: Intelligent orchestration for "how to do it safely" - resolves dependencies and conflicts  
- **Executor Layer**: Actual execution for "do it" - manages backups, monitors progress, handles errors

```go
// Clean, unified API
batch := synthfs.NewBatch(fs, registry)

// Operations declare prerequisites automatically  
_, err := batch.CreateFile("project/src/main.go", content)

// Dependency resolution and execution happen transparently
result, err := batch.Run()
```

## 🔧 **Configuration and Execution Modes**

### Execution Modes

```go
// Standard execution (no backup)
result, err := batch.Run()

// Restorable execution (10MB backup budget)
result, err := batch.RunRestorable()

// Custom backup budget
result, err := batch.RunRestorableWithBudget(100) // 100MB

// Advanced options
opts := map[string]interface{}{
    "restorable": true,
    "max_backup_size_mb": 25,
}
result, err := batch.RunWithOptions(opts)
```

### Configuration Options

```go
batch := synthfs.NewBatch(fs, registry).
    WithContext(ctx).                    // Set execution context
    WithLogger(logger).                  // Custom logging
    WithFileSystem(customFS)             // Custom filesystem
```

### Result Handling

```go
result, err := batch.RunRestorable()
if err != nil {
    return fmt.Errorf("execution failed: %w", err)
}

// Check overall success
if !result.IsSuccess() {
    fmt.Printf("Batch failed: %v\n", result.GetError())
    
    // Check what operations completed
    fmt.Printf("Completed: %d operations\n", len(result.GetOperations()))
    
    // Check restoration options
    if restoreOps := result.GetRestoreOps(); len(restoreOps) > 0 {
        fmt.Printf("Can restore %d operations\n", len(restoreOps))
    }
}

fmt.Printf("Execution took: %v\n", result.GetDuration())
```

## 📚 **Documentation**

### Comprehensive Guides

- **[Operations Reference](docs/operations.txxt)** - Complete guide to all filesystem operations and target types
- **[Correctness Model](docs/correctness.txxt)** - Understanding SynthFS safety guarantees and limitations  
- **[Batch Architecture](docs/batch.txxt)** - Deep dive into batch/pipeline/execution system
- **[Introduction](docs/intro-to-synthfs.txxt)** - Core concepts and philosophy

### Safety and Guarantees

SynthFS provides **best-effort optimistic** semantics designed for controlled environments:

- ✅ **Multi-layer validation**: Operations validated at creation, batch composition, and execution
- ✅ **Change detection**: MD5 checksums detect concurrent file modifications
- ✅ **Automatic dependencies**: Parent directories and prerequisites resolved automatically
- ✅ **Budget-controlled backups**: Predictable memory usage with fail-fast behavior
- ⚠️ **Best effort**: Not suitable for high-concurrency or mission-critical scenarios

## 💻 **Development**

### Building and Testing

```bash
# Build all packages
./scripts/build

# Run tests with coverage
./scripts/test-with-coverage

# Run linting
./scripts/lint

# Generate and view coverage report
./scripts/test-with-coverage
open coverage.html
```

### Project Structure

```
pkg/synthfs/                 # Main library
├── core/                   # Shared interfaces and types
├── batch/                  # Batch orchestration
├── execution/              # Pipeline and execution engine
├── operations/             # Individual operation implementations  
├── targets/                # Filesystem item types
├── filesystem/             # Filesystem abstraction layer
└── testutil/               # Testing utilities
```

## 📋 **Advanced Usage**

### Custom Filesystem Implementation

```go
type MyFileSystem struct {
    // Your implementation
}

func (fs *MyFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
    // Custom write logic
    return nil
}

// Use with SynthFS
batch := synthfs.NewBatch(&MyFileSystem{}, registry)
```

### Project Scaffolding Example

```go
func CreateProjectStructure(projectName string) error {
    fs := filesystem.NewOSFileSystem(".")
    registry := operations.NewFactory()
    batch := synthfs.NewBatch(fs, registry)
    
    // Create project structure
    _, err := batch.CreateDir(projectName)
    if err != nil {
        return err
    }
    
    // Create source directories and files
    _, err = batch.CreateFile(projectName+"/main.go", []byte(mainTemplate))
    if err != nil {
        return err
    }
    
    _, err = batch.CreateFile(projectName+"/go.mod", []byte(goModTemplate))
    if err != nil {
        return err
    }
    
    // Copy template files
    _, err = batch.Copy("templates/README.md", projectName+"/README.md")
    if err != nil {
        return err
    }
    
    // Execute with automatic dependency resolution
    result, err := batch.RunRestorable()
    if err != nil {
        return fmt.Errorf("project creation failed: %w", err)
    }
    
    if !result.IsSuccess() {
        return fmt.Errorf("project creation incomplete: %v", result.GetError())
    }
    
    return nil
}
```

### Testing with SynthFS

```go
func TestApplicationConfig(t *testing.T) {
    // Use test filesystem for controlled testing
    testFS := testutil.NewTestFileSystem()
    registry := operations.NewFactory()
    
    batch := synthfs.NewBatch(testFS, registry)
    
    // Create test scenario
    _, err := batch.CreateFile("config.yaml", []byte("test: true"))
    require.NoError(t, err)
    
    // Execute and verify
    result, err := batch.Run()
    require.NoError(t, err)
    require.True(t, result.IsSuccess())
    
    // Verify filesystem state
    assert.True(t, testutil.FileExists(t, testFS, "config.yaml"))
}
```

## 🛡️ **Safety Considerations**

SynthFS is designed for **controlled environments** where applications have exclusive access to target filesystem areas:

### ✅ **Recommended Use Cases**

- Application configuration setup
- Development tooling and scaffolding  
- Deployment scripts in isolated environments
- Testing with controlled filesystem state

### ⚠️ **Not Recommended**

- High-concurrency environments
- Mission-critical data with external concurrent access
- Scenarios requiring strict transactional guarantees
- Operations on files modified by other processes

### Memory Usage

- Default backup budget: 10MB
- Configurable up to practical memory limits
- Operations fail fast when budget exceeded
- Budget applies per-batch, not globally

## 📄 **License**

MIT License - see LICENSE file for details

## 🤝 **Contributing**

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Run tests: `./scripts/test`
4. Run linting: `./scripts/lint`
5. Commit changes (`git commit -m 'Add amazing feature'`)
6. Push to branch (`git push origin feature/amazing-feature`)
7. Submit a pull request

See the development scripts in `scripts/` directory for building, testing, and linting.

---

**SynthFS makes filesystem operations predictable, testable, and safe.** 🚀
