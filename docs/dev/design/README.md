# Porting Fsynth to Go: Development Brief

go-synthfs is a go library that creates "synthetic" file system operations, that is, a queue, list of operations to be done.
at a later time, one can queue and process those.

now, it's important that a system such as this can never be correct, as this would be a future write ahead log for a file system, not possible.

the reason for this, is to allow applications to deal with simple file changes in an abstract manner (ie. will create the files x at z)
and thoroughly test their application like, isolating the "realization" the conversion from the planned file system change to the actual change.

This gives application a chance to work with pure functions and isolate side effects.

This is not however

    - a library that can deal with concurrency (if between starting to generate operations and finishing them the fs changes, all bets are off)
    - provide real guarantees or constraints

This library is a go port of my lua library: <https://github.com/arthur-debert/fsynth.lua//>
(also checked out locally in the dev env at /Users/adebert/h/lua/fsynth.lua)
consulting that for information, doc or other questions is useful.

However we are in no way looking for a 1 to 1 port, we want idiomatic go, and code that uses the best patterns for the platform.

## Core Architecture Decisions

### 1. **Build on Go's Standard io/fs Package**

- Build around `io/fs.FS` interfaces for read operations and extend with custom interfaces for write operations
- **Avoid Afero dependency** - the ecosystem is moving toward `io/fs` as the standard, and Afero has known issues with MemMapFs and other components
- Design thin interfaces following Go's philosophy: prefer many small interfaces over large ones

```go
// Build on io/fs foundation
type ReadFS = fs.FS

type WriteFS interface {
    WriteFile(name string, data []byte, perm fs.FileMode) error
    MkdirAll(path string, perm fs.FileMode) error
    Remove(name string) error
    RemoveAll(name string) error
}

type FileSystem interface {
    ReadFS
    WriteFS
}

type OperationID string

type Operation interface {
    ID() OperationID
    Execute(ctx context.Context, fs FileSystem) error
    Validate(ctx context.Context, fs FileSystem) error
    Dependencies() []OperationID // Operations that must run before this one
    Conflicts() []OperationID    // Operations that cannot run concurrently
    Rollback(ctx context.Context, fs FileSystem) error
    Describe() OperationDesc
}
```

### 2. **Operation Dependency Management**

- Support operation dependencies to ensure correct execution order
- Detect circular dependencies during queue resolution
- Enable conflict detection for safe concurrent execution

```go
type Queue interface {
    Add(ops ...Operation) error
    Resolve() error  // Resolve dependencies and detect conflicts
    Validate(ctx context.Context, fs FileSystem) error
    Execute(ctx context.Context, fs FileSystem, opts ...ExecuteOption) *Result
}

// Example usage
queue.Add(
    ops.CreateDir("/tmp/project").WithID("create-dir"),
    ops.CreateFile("/tmp/project/config.json", data).
        WithDependency("create-dir"), // Ensures dir exists first
)
```

### 3. **Enhanced Context-Driven Design**

- Use `context.Context` throughout for cancellation, timeouts, and tracing
- Support context cancellation during long-running batch operations
- Add progress reporting through context values

```go
type Executor struct{}

func (e *Executor) Execute(ctx context.Context, queue Queue, opts ...ExecuteOption) *Result
func (e *Executor) ExecuteWithProgress(ctx context.Context, queue Queue, reporter ProgressReporter, opts ...ExecuteOption) *Result
```

### 4. **Functional Options Pattern**

Replace Lua's table-based configuration with Go's functional options:

```go
type ExecuteOption func(*ExecuteConfig)

func WithDryRun(enabled bool) ExecuteOption    // Applications can implement their own dry-run logic
func WithTransactional(enabled bool) ExecuteOption
func WithConcurrency(workers int) ExecuteOption
func WithProgressReporter(reporter ProgressReporter) ExecuteOption

// Usage
result := executor.Execute(ctx, queue, 
    WithDryRun(true),        // For application-level simulation
    WithTransactional(true),
    WithConcurrency(4),
)
```

### 5. **Type-Safe Operation Builders**

Use Go's type system to prevent invalid operations at compile time:

```go
type OperationBuilder struct{}

func (b *OperationBuilder) CreateFile(path string, content []byte, opts ...FileOption) *FileOperation
func (b *OperationBuilder) CreateDir(path string, opts ...DirOption) *DirOperation
func (b *OperationBuilder) Copy(src, dst string, opts ...CopyOption) *CopyOperation

// Chainable for complex operations with dependencies
queue.Add(
    ops.CreateDir("/tmp/project").
        WithID("project-dir").
        WithMode(0755).
        WithParents(true),
    ops.CreateFile("/tmp/project/config.json", configData).
        WithID("config-file").
        WithDependency("project-dir"),
)
```

## Key Go Idioms to Adopt

### 1. **Enhanced Error Handling**

- Return explicit errors instead of Lua's status + error pattern
- Use error wrapping with `fmt.Errorf("operation failed: %w", err)`
- Provide rich error types for different failure modes including dependency errors

```go
type ValidationError struct {
    Operation Operation
    Reason    string
    Cause     error
}

type DependencyError struct {
    Operation    Operation
    Dependencies []OperationID
    Missing      []OperationID
}

type ConflictError struct {
    Operation Operation
    Conflicts []OperationID
}

func (e *ValidationError) Error() string { /* */ }
func (e *ValidationError) Unwrap() error { return e.Cause }
```

### 2. **Structured Results with Rollback Support**

Replace Lua tables with structured types:

```go
type Result struct {
    Success     bool
    Operations  []OperationResult
    Duration    time.Duration
    Errors      []error
    Rollback    func(context.Context) error // Rollback function for failed transactions
}

type OperationResult struct {
    OperationID OperationID
    Operation   Operation
    Status      OperationStatus
    Error       error
    Metrics     OperationMetrics
    Duration    time.Duration
}
```

### 3. **Progress Reporting and Streaming**

For large operation sets, support progress reporting and streaming results:

```go
type ProgressReporter interface {
    OnStart(op Operation)
    OnProgress(op Operation, current, total int64)
    OnComplete(op Operation, result OperationResult)
}

func (e *Executor) ExecuteStream(ctx context.Context, queue Queue) <-chan OperationResult
```

## Integration with Go Ecosystem

### 1. **Testing Integration**

- Provide test helpers that work with Go's testing package
- Include golden file testing utilities
- Support test fixtures and assertions with `io/fs` compatible test filesystems

```go
func TestOperations(t *testing.T) {
    // Use standard library testing/fstest
    fs := fstest.MapFS{
        "existing.txt": &fstest.MapFile{Data: []byte("content")},
    }
    
    queue := synthfs.NewQueue()
    queue.Add(ops.CreateFile("test.txt", []byte("content")))
    
    result := executor.Execute(context.Background(), queue, WithFileSystem(fs))
    
    assert.True(t, result.Success)
}
```

### 2. **Observability**

- Support structured logging (logrus, zap)
- Provide OpenTelemetry tracing hooks
- Emit metrics compatible with Prometheus

```go
func WithLogger(logger Logger) ExecuteOption
func WithTracer(tracer trace.Tracer) ExecuteOption
```

### 3. **CLI Integration**

Design operations to be serializable for CLI tools:

```go
type SerializableOperation interface {
    Operation
    MarshalJSON() ([]byte, error)
    UnmarshalJSON([]byte) error
}

// Enable: synthfs plan.json --dry-run
```

## Implementation Strategy

### Phase 1: Core Foundation

1. Define core interfaces based on `io/fs` (`ReadFS`, `WriteFS`, `FileSystem`)
2. Implement basic operations (Create, Delete, Copy, Move) with dependency support
3. Build simple executor with dependency resolution
4. Create operation builders with chainable API

### Phase 2: Advanced Features

1. Add dependency resolution with topological sorting
2. Implement conflict detection for concurrent operations
3. Add transactional rollback support
4. Implement operation validation with rich error types

### Phase 3: Ecosystem Integration

1. Create `io/fs` compatible filesystem implementations
2. Implement testing utilities with `testing/fstest` integration
3. Create CLI tool with operation serialization
4. Add progress reporting and streaming execution

### Phase 4: Production Features

1. Add observability hooks (logging, tracing, metrics)
2. Performance optimizations and benchmarks
3. Add comprehensive examples and documentation
4. Create integration guides for popular frameworks

## Key Differences from Lua Version

1. **Type Safety**: Leverage Go's type system for compile-time safety
2. **Dependency Management**: Explicit operation dependencies with conflict detection
3. **Standard Library Integration**: Built on `io/fs` instead of custom abstractions
4. **Concurrency**: Support concurrent operation execution with conflict resolution
5. **Context Support**: Built-in cancellation, timeout, and progress reporting
6. **Resource Management**: Proper cleanup with defer and context cancellation
7. **Error Handling**: Rich, structured error types for dependencies and conflicts

## Success Metrics

- **Standard Library Compatibility**: Can work with any `io/fs.FS` implementation
- **Performance**: Efficient dependency resolution and concurrent execution
- **Developer Experience**: Clear APIs with comprehensive error messages and documentation
- **Testing**: Full test coverage with realistic integration tests using `testing/fstest`
- **Production Ready**: Comprehensive observability and error handling

This approach will create a library that feels natural to Go developers while preserving Fsynth's core value proposition of predictable, testable filesystem operations, enhanced with robust dependency management and conflict resolution.
