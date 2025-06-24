# Design Decisions

## Imperative API Design (2024-12)

### Problem Statement

The current synthfs API, while powerful, has a steep learning curve and requires explicit management of:

- Operation IDs and dependencies
- Builder patterns with method chaining
- Manual queue and executor setup
- Separate validation and execution phases

This creates barriers for adoption, especially for simple use cases where users just want to perform a sequence of filesystem operations.

### Design Goals

1. **Simplicity First**: Make simple operations simple to express
2. **Fail Fast**: Validate operations immediately when added to batch
3. **Automatic Dependencies**: Infer dependencies from filesystem paths
4. **Familiar Patterns**: Follow Go idioms and database-like transaction patterns
5. **Preserve Power**: Keep all advanced features available under the hood

### API Design

#### Core Concept: Batch

The `Batch` is the central abstraction - a collection of filesystem operations that can be validated and executed as a unit.

```go
// Simple usage
batch := synthfs.NewBatch()
batch.CreateDir("code")
batch.Copy("config.yaml", "code/config.yaml.bak")
result, err := batch.Execute()
```

#### Key Features

1. **Validate-as-you-go**: Each method validates the operation immediately
2. **Automatic dependency resolution**: Analyzes paths to detect missing parent directories
3. **Immutable operations**: Operations are validated and frozen when added
4. **Rich return objects**: Operations return metadata and can be inspected

#### Method Signatures

```go
type Batch struct {
    operations []Operation
    fs         FileSystem
    ctx        context.Context
}

// Core methods
func (b *Batch) CreateDir(path string, mode ...fs.FileMode) (Operation, error)
func (b *Batch) CreateFile(path string, content []byte, mode ...fs.FileMode) (Operation, error)
func (b *Batch) Copy(src, dst string) (Operation, error)
func (b *Batch) Move(src, dst string) (Operation, error)
func (b *Batch) Delete(path string) (Operation, error)
func (b *Batch) Execute() (*Result, error)

// Configuration
func (b *Batch) WithFileSystem(fs FileSystem) *Batch
func (b *Batch) WithContext(ctx context.Context) *Batch
```

### Implementation Strategy

#### Phase 1: Core Batch Structure

- Create `Batch` struct with basic interface
- Implement operation addition with immediate validation
- Bridge to existing Operation/Queue/Executor infrastructure

#### Phase 2: Automatic Dependency Resolution

- Analyze filesystem paths for missing parent directories
- Auto-generate CreateDir operations for missing parents
- Handle dependency ordering automatically

#### Phase 3: Rich Operation Objects

- Return meaningful operation objects from batch methods
- Provide operation metadata and inspection capabilities
- Support operation customization where needed

#### Phase 4: Advanced Features

- Context-aware operations with timeouts
- Custom filesystem support (mock, testing, etc.)
- Progress reporting and cancellation

### Comparison with Current API

| **Current API** | **New Imperative API** |
|----------------|----------------------|
| `ops.NewCreateFile("file.txt", content, 0644).WithID("f1").WithDependency("d1")` | `batch.CreateFile("file.txt", content)` |
| Manual dependency management | Automatic path-based dependencies |
| Separate validation phase | Validate-as-you-go |
| Explicit queue/executor setup | Built-in execution |
| Builder pattern complexity | Direct method calls |

### Benefits

1. **Lower barrier to entry**: Simple operations are simple to express
2. **Fewer bugs**: Immediate validation catches errors early
3. **Less boilerplate**: No manual ID/dependency management for common cases
4. **Better error messages**: Context-aware validation with meaningful errors
5. **Familiar patterns**: Similar to SQL transactions or HTTP request builders

### Migration Strategy

The imperative API will be built on top of the existing infrastructure:

- `Batch` internally uses `Operation`, `Queue`, and `Executor`
- Existing explicit API remains available for advanced use cases
- Gradual migration path for complex applications
- Full backward compatibility maintained

### Future Considerations

1. **Serialization**: Batch could support plan export/import
2. **Parallel execution**: Batch could optimize independent operations
3. **Streaming**: Large operations could support progress callbacks
4. **Extension points**: Custom operation types could plug into batch system

### Decision Rationale

This design prioritizes **developer experience** over **API purity**. While the current explicit API is more "correct" from a functional programming perspective, the imperative API reduces cognitive load for the majority of use cases while preserving all the power for advanced scenarios.

The auto-dependency resolution is particularly valuable because filesystem operations naturally have path-based dependencies that are predictable and can be inferred automatically.
