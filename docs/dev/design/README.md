# Porting Fsynth to Go: Development Brief

go-synthfs  is a  go library that creates "synthetic" file system operations, that is , a queue, list of operations to be done.
at a later time, one can queue an processes those.

now, it's important that a system such as this can never be correct, as this would be a future write ahead log for a file system, not possible.

the reason for this , is to allow applications to deal with simple file changes in an abstrct manner (ie. will create the fils x at z)
and throughly test their application like , isoilating the "realization" the convertion from the planned file system change to thee actual change.

This gives application a chance to work with pure functions and isolatgee side effects.

This is not however

    - a library that can deal with concurrency (if bettween starting to generate operations and finishing them the fs changes, all bets are off)
    - provide real guarantees or constraints

This library is a go port of my lua library: <https://github.com/arthur-debert/fsynth.lua//>
(also checked out locally in the dev env at /Users/adebert/h/lua/fsynth.lua)
consulting that for information, doc or other questions is usefull.

However we are in way looking for a 1 to 1 port, we want idyomattic go, and code that uses the best patterns for the platform, for example Afero compatiblfor example Afero compatible

## Core Architecture Decisions

### 1. **Leverage Go's Interface System**

- Build around `io/fs.FS` interfaces for read operations and extend with custom interfaces for write operations
- Make the library **compatible with Afero** by accepting `afero.Fs` interfaces
- Design thin interfaces following Go's philosophy: prefer many small interfaces over large ones

```go
type FileSystemOperator interface {
    afero.Fs  // Embed existing ecosystem interfaces
}

type Operation interface {
    Execute(ctx context.Context, fs FileSystemOperator) error
    Validate(fs FileSystemOperator) error
    Describe() OperationDesc
    Rollback(ctx context.Context, fs FileSystemOperator) error
}
```

### 2. **Embrace Context-Driven Design**

- Use `context.Context` throughout for cancellation, timeouts, and tracing
- Support context cancellation during long-running batch operations

```go
type Executor struct{}

func (e *Executor) Execute(ctx context.Context, queue *Queue, opts ...ExecuteOption) *Result
```

### 3. **Functional Options Pattern**

Replace Lua's table-based configuration with Go's functional options:

```go
type ExecuteOption func(*ExecuteConfig)

func WithDryRun(enabled bool) ExecuteOption
func WithTransactional(enabled bool) ExecuteOption
func WithConcurrency(workers int) ExecuteOption

// Usage
result := executor.Execute(ctx, queue, 
    WithDryRun(true),
    WithTransactional(true),
)
```

### 4. **Type-Safe Operation Builders**

Use Go's type system to prevent invalid operations at compile time:

```go
type OperationBuilder struct{}

func (b *OperationBuilder) CreateFile(path string, content []byte, opts ...FileOption) *FileOperation
func (b *OperationBuilder) CreateDir(path string, opts ...DirOption) *DirOperation
func (b *OperationBuilder) Copy(src, dst string, opts ...CopyOption) *CopyOperation

// Chainable for complex operations
queue.Add(
    ops.CreateDir("/tmp/project").
        WithMode(0755).
        WithParents(true),
)
```

## Key Go Idioms to Adopt

### 1. **Error Handling**

- Return explicit errors instead of Lua's status + error pattern
- Use error wrapping with `fmt.Errorf("operation failed: %w", err)`
- Provide rich error types for different failure modes

```go
type ValidationError struct {
    Operation Operation
    Reason    string
    Cause     error
}

func (e *ValidationError) Error() string { /* */ }
func (e *ValidationError) Unwrap() error { return e.Cause }
```

### 2. **Structured Results**

Replace Lua tables with structured types:

```go
type Result struct {
    Success    bool
    Operations []OperationResult
    Duration   time.Duration
    Errors     []error
}

type OperationResult struct {
    Operation Operation
    Status    OperationStatus
    Error     error
    Metrics   OperationMetrics
}
```

### 3. **Streaming and Channels**

For large operation sets, support streaming results:

```go
func (e *Executor) ExecuteStream(ctx context.Context, queue *Queue) <-chan OperationResult
```

## Integration with Go Ecosystem

### 1. **Testing Integration**

- Provide test helpers that work with Go's testing package
- Include golden file testing utilities
- Support test fixtures and assertions

```go
func TestOperations(t *testing.T) {
    fs := afero.NewMemMapFs()
    queue := fsynth.NewQueue()
    
    queue.Add(ops.CreateFile("test.txt", []byte("content")))
    
    result := executor.Execute(context.Background(), queue, WithFilesystem(fs))
    
    assert.True(t, result.Success)
    assert.FileExists(t, fs, "test.txt")
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
type Queue struct{}

func (q *Queue) MarshalJSON() ([]byte, error)
func (q *Queue) UnmarshalJSON(data []byte) error

// Enable: fsynth plan.json --dry-run
```

## Implementation Strategy

### Phase 1: Core Foundation

1. Define core interfaces (`Operation`, `Queue`, `Executor`)
2. Implement basic operations (Create, Delete, Copy, Move)
3. Build simple executor with dry-run support

### Phase 2: Advanced Features

1. Add transactional rollback
2. Implement operation validation
3. Add concurrent execution with worker pools

### Phase 3: Ecosystem Integration

1. Add Afero compatibility layer
2. Implement testing utilities
3. Create CLI tool and examples

### Phase 4: Production Features

1. Add observability hooks
2. Implement streaming execution
3. Performance optimizations and benchmarks

## Key Differences from Lua Version

1. **Type Safety**: Leverage Go's type system for compile-time safety
2. **Concurrency**: Support concurrent operation execution with goroutines
3. **Context Support**: Built-in cancellation and timeout support
4. **Interface Integration**: Work seamlessly with existing Go filesystem libraries
5. **Resource Management**: Proper cleanup with defer and context cancellation
6. **Error Handling**: Rich, structured error types instead of string-based errors

## Success Metrics

- **Ecosystem Fit**: Can be used as a drop-in replacement in Afero-based codebases
- **Performance**: Faster than sequential os operations for bulk operations
- **Developer Experience**: Clear APIs with good error messages and documentation
- **Testing**: Comprehensive test coverage with realistic integration tests

This approach will create a library that feels natural to Go developers while preserving Fsynth's core value proposition of predictable, testable filesystem operations.
