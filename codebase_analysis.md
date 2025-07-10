# SynthFS Codebase Analysis

## Overview

SynthFS is a Go library for **lazy filesystem operations** with automatic dependency resolution and best-effort rollback. The codebase uses a sophisticated wrapper/adapter pattern to solve Go's circular dependency limitations while maintaining clean separation of concerns.

## Architecture Pattern: Wrapper/Adapter System

The entire codebase is built around avoiding Go's circular import restrictions through a **wrapper pattern**:

1. **Domain packages** (batch/, execution/, operations/, etc.) use generic `interface{}` types
2. **Root package** (pkg/synthfs/) provides concrete type definitions and public API
3. **Wrappers/Adapters** bridge between concrete and generic types
4. **Clean dependency flow**: root → domain packages (never the reverse)

## Package Structure & Relationships

### Core Package (`pkg/synthfs/core/`)

**Purpose**: Shared interfaces, types, and abstractions used across all packages

**Key Components**:

- `types.go`: Core types like `OperationID`, `OperationDesc`, `BackupData`, `PathStateType`
- `interfaces.go`: Core interfaces like `OperationMetadata`, `DependencyAware`, `ExecutableV2`
- `prerequisites.go`: Prerequisite system for automatic dependency resolution
- `context.go`: `ExecutionContext` and `Logger` interfaces for execution
- `events.go`: Event system for operation monitoring
- `execution_types.go`: `PipelineOptions`, `OperationResult`, `Result`

**Why it exists**: Provides shared abstractions without creating circular dependencies. All packages can import core without importing each other.

### Execution Package (`pkg/synthfs/execution/`)

**Purpose**: Core execution engine with pipeline management and operation execution

**Key Components**:

- `executor.go`: Main execution engine that runs pipelines of operations
- `pipeline.go`: Pipeline implementation with dependency resolution and prerequisite handling
- `prerequisite_resolver.go`: Resolves prerequisites like parent directories
- `state.go`: Path state tracking for projected filesystem state

**Relationships**:

- Uses `core/` interfaces and types
- Provides generic `interface{}` based interfaces to avoid circular deps
- Wrapped by `pkg/synthfs/executor.go` and `pkg/synthfs/pipeline.go`

### Batch Package (`pkg/synthfs/batch/`)

**Purpose**: High-level batch operation orchestration

**Key Components**:

- `batch.go`: Main batch implementation with operation creation methods
- `interfaces.go`: Batch and Result interfaces
- `result.go`: Result implementations for batch execution

**Relationships**:

- Uses `execution/` for actual operation execution
- Uses `core/` for shared types and interfaces
- Wrapped by `pkg/synthfs/batch.go`

**The Confusion**: You noticed lots of batch code in both root and `batch/`. This is the wrapper pattern:

- `pkg/synthfs/batch.go` (219 lines) - **Public API wrapper**
- `pkg/synthfs/batch/batch.go` (975 lines) - **Actual implementation**

### Pipeline Package (Also follows the wrapper pattern!)

**Purpose**: Operations orchestration and dependency management

**The Pattern**:

- `pkg/synthfs/pipeline.go` (86 lines) - **Public API wrapper**  
- `pkg/synthfs/execution/pipeline.go` (490 lines) - **Actual implementation**

**Key Components**:

- **Dependency Resolution**: Uses topological sorting to order operations based on dependencies
- **Prerequisite Resolution**: Automatically creates missing parent directories and satisfies other prerequisites
- **Validation**: Checks that all operations in the pipeline are valid and dependencies exist
- **Operation Management**: Manages sequences of operations with conflict detection

**Pipeline's Role in the Architecture**:
Pipeline sits between individual operations and batch execution. It's the "orchestration layer" that:

1. **Collects operations** from batch creation methods
2. **Resolves prerequisites** (like ensuring parent directories exist)
3. **Orders operations** based on dependencies using topological sort
4. **Validates the entire sequence** before execution
5. **Feeds the ordered, validated operations** to the executor

**Relationships**:

- Uses `core/` interfaces and types
- Provides generic `interface{}` based interfaces to avoid circular deps
- Wrapped by `pkg/synthfs/pipeline.go`
- Used by both `batch/` and `execution/` packages

### Operations Package (`pkg/synthfs/operations/`)

**Purpose**: Individual operation implementations (CreateFile, Copy, Move, etc.)

**Key Components**:

- `factory.go`: Operation factory for creating operations by type
- `base.go`: Base operation implementation with common functionality
- `create.go`: File and directory creation operations
- `copy_move.go`: Copy and move operations
- `delete.go`: Delete operations
- `symlink.go`: Symlink operations
- `archive.go`: Archive/unarchive operations

**Relationships**:

- Uses `core/` for shared types
- Uses `targets/` for filesystem item types
- Wrapped by `pkg/synthfs/operations_adapter.go`

### Targets Package (`pkg/synthfs/targets/`)

**Purpose**: Filesystem item types (File, Directory, Symlink, Archive)

**Key Components**:

- `file.go`: FileItem with content and mode
- `directory.go`: DirectoryItem with mode
- `symlink.go`: SymlinkItem with target
- `archive.go`: ArchiveItem with format and sources

**Relationships**:

- Pure data structures with no external dependencies
- Wrapped by `pkg/synthfs/items.go`

### Root Package (`pkg/synthfs/`)

**Purpose**: Public API with concrete types and wrappers

**Key Files**:

- `batch.go`: Wrapper around `batch/batch.go`
- `pipeline.go`: Wrapper around `execution/pipeline.go`
- `executor.go`: Wrapper around `execution/executor.go`
- `operations_adapter.go`: Adapts operations package to main package interfaces
- `registry.go`: Operation registry wrapper
- `types.go`: Public type definitions and aliases

## The "Shared Code" Question

You asked about shared code in `core/` vs elsewhere. Here's the pattern:

### Always in Core

- **Basic types**: `OperationID`, `OperationDesc`, `OperationStatus`
- **Interfaces**: `OperationMetadata`, `DependencyAware`, `ExecutableV2`
- **Execution context**: `ExecutionContext`, `Logger`, `EventBus`
- **Prerequisites**: `Prerequisite`, `PrerequisiteResolver`

### Never in Core

- **Concrete implementations**: Actual operation logic, filesystem operations
- **Package-specific interfaces**: Those that would create circular deps
- **Complex business logic**: Stays in domain packages

### The Rule

If multiple packages need it AND it won't create circular dependencies → Core
If it's implementation-specific OR would create cycles → Domain package

## Data Flow Example

Here's how a simple `batch.CreateFile()` flows through the system:

1. **User calls**: `batch.CreateFile("file.txt", content)`
2. **Root wrapper** (`pkg/synthfs/batch.go`): Delegates to `batch.Batch.CreateFile()`
3. **Batch implementation** (`pkg/synthfs/batch/batch.go`):
   - Creates operation via registry
   - Wraps in adapters for interface compatibility
4. **Operation creation** (`pkg/synthfs/operations/factory.go`): Creates `CreateFileOperation`
5. **Pipeline orchestration** (`pkg/synthfs/execution/pipeline.go`):
   - Resolves prerequisites (parent directories)
   - Orders operations using topological sort
   - Validates the entire sequence
6. **Execution** (`pkg/synthfs/execution/executor.go`):
   - Executes operations in dependency order from pipeline
7. **Result** flows back through wrappers to user

## Interface Hierarchy

```
core.OperationMetadata
├── core.DependencyAware
├── core.ExecutableV2
└── synthfs.Operation (main package)
    ├── synthfs.Executable
    └── Additional methods (GetItem, Prerequisites, etc.)
```

## Why This Architecture?

1. **Avoids Circular Dependencies**: Go's strict import rules are satisfied
2. **Clean Separation**: Each package has a single responsibility
3. **Testable**: Domain packages can be tested independently
4. **Flexible**: Easy to swap implementations behind interfaces
5. **Backward Compatible**: Public API remains stable while internals evolve

## Common Patterns

### Wrapper Pattern

```go
// Root package (public API)
type Batch struct {
    impl batch.Batch  // Delegates to domain package
}

// Domain package (implementation)  
type BatchImpl struct {
    operations []interface{}  // Uses interface{} to avoid cycles
}

// Same pattern for Pipeline
type Pipeline interface {      // Root package interface
    Add(ops ...Operation) error
    Operations() []Operation
    Resolve() error
    Validate(ctx context.Context, fs FileSystem) error
}

type pipelineAdapter struct {  // Root package adapter
    pipeline execution.Pipeline  // Delegates to execution package
}
```

### Adapter Pattern

```go
// Adapts operations package to main package interface
type OperationsPackageAdapter struct {
    opsOperation operations.Operation
}

func (a *OperationsPackageAdapter) Execute(ctx context.Context, fsys FileSystem) error {
    return a.opsOperation.Execute(ctx, fsys)
}
```

### Interface{} Pattern

```go
// Domain packages use interface{} to avoid circular deps
type Pipeline interface {
    Add(ops ...interface{}) error
    Operations() []interface{}
}
```

This architecture allows SynthFS to have a clean, modular design while working within Go's import restrictions. The "confusion" you experienced is actually the sophisticated solution to a complex architectural challenge!

## Simple Responsibility Split

### 🎯 **Batch** = "What to do"

**Responsibility**: User-facing API for **creating** filesystem operations

```go
batch := synthfs.NewBatch(fs, registry)
batch.CreateFile("deep/nested/file.txt", content)  // User says "create this file"
batch.Copy("source.txt", "backup/source.txt")      // User says "copy this"
batch.Delete("old.txt")                             // User says "delete this"
```

**Batch's job**:

- Provide simple methods like `CreateFile()`, `Copy()`, `Move()`
- Validate operations as they're added
- Collect operations into a list
- **Doesn't care about order or dependencies**

---

### 🔧 **Pipeline** = "How to do it safely"

**Responsibility**: **Orchestrating** operations for safe execution

```go
// Pipeline takes batch's operations and figures out:
// 1. "file.txt needs parent directory 'deep/nested/' to exist first"
// 2. "Let me create a CreateDir operation for that"
// 3. "CreateDir must run BEFORE CreateFile"
// 4. "Final order: CreateDir → CreateFile → Copy → Delete"
```

**Pipeline's job**:

- **Resolve prerequisites**: Auto-create missing parent directories
- **Order operations**: Use dependency sorting (topological sort)
- **Validate everything**: Check the whole sequence makes sense
- **Detect conflicts**: Prevent operations that would interfere
- **Doesn't actually execute anything**

---

### ⚡ **Executor** = "Actually do it"

**Responsibility**: **Running** the operations on the filesystem

```go
// Executor takes pipeline's ordered operations and:
// 1. CreateDir("deep/nested/") → Actually creates the directory
// 2. CreateFile("deep/nested/file.txt") → Actually writes the file
// 3. Copy("source.txt", "backup/source.txt") → Actually copies
// 4. Delete("old.txt") → Actually deletes
```

**Executor's job**:

- **Execute operations** one by one in order
- **Handle failures**: Stop and rollback if something goes wrong
- **Manage backups**: Create restore operations if needed
- **Report results**: Success/failure for each operation

---

## Real-World Analogy

Think of building a house:

- **Batch** = **Architect**: "I want a foundation, walls, roof, plumbing"
- **Pipeline** = **Project Manager**: "Foundation first, then walls, then roof. Plumbing needs walls first. Here's the schedule."
- **Executor** = **Construction Crew**: "Actually pouring concrete, building walls, installing roof"

## Data Flow

```
User API Call
     ↓
┌─────────┐    Operations    ┌──────────┐    Ordered &     ┌──────────┐
│  Batch  │ ───────────────→ │ Pipeline │ ──────────────→  │ Executor │
│         │    (what to do)   │          │  Prerequisites   │          │
│"Create" │                   │"Order &  │    Resolved      │"Actually │
│"Copy"   │                   │ Resolve" │                  │"Execute" │
│"Delete" │                   │          │                  │          │
└─────────┘                   └──────────┘                  └──────────┘
```

## Why Split This Way?

1. **Batch**: Simple API - users don't think about dependencies
2. **Pipeline**: Smart orchestration - handles complexity automatically  
3. **Executor**: Clean execution - focused on actually doing the work

Each has **one clear job**, making the code easier to understand, test, and maintain!
