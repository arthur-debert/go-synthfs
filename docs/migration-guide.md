# Migration Guide: Prerequisite-Based Execution

This guide explains how to migrate from the old hardcoded batch logic to the new prerequisite-based execution system.

## Overview

SynthFS has introduced a new execution model that uses **prerequisite resolution** instead of hardcoded parent directory creation logic. This provides better extensibility, testability, and maintainability.

## Migration Phases

### Current State (Phase 5)

- **Old Batch** (default): Uses hardcoded parent directory creation logic
- **SimpleBatch**: Uses prerequisite resolution instead of hardcoded logic  
- **Migration Options**: Allow gradual transition

### Key Differences

| Feature | Old Batch | SimpleBatch |
|---------|-----------|-------------|
| Parent Directory Creation | Hardcoded in batch logic | Via prerequisite resolution |
| Extensibility | Limited | High - add new prerequisite types |
| Operation Types | Hardcoded strings | Declared by operations |
| Dependency Management | Manual | Automatic via prerequisites |

## Usage Examples

### Current (Backward Compatible)

```go
// Uses old hardcoded logic (default)
batch := synthfs.NewBatch()
batch.CreateFile("deep/path/file.txt", []byte("content"))
result, err := batch.Run() // Parent dirs created via hardcoded logic
```

### Opt-in to Prerequisite Resolution

```go
// Option 1: Use existing batch with prerequisite resolution
batch := synthfs.NewBatch()
batch.CreateFile("deep/path/file.txt", []byte("content"))
result, err := batch.RunWithPrerequisites() // Uses prerequisite resolution

// Option 2: Use simplified batch constructor
batch := synthfs.NewSimpleBatch()
batch.CreateFile("deep/path/file.txt", []byte("content"))
result, err := batch.Run() // Prerequisite resolution enabled by default

// Option 3: Use migration constructor
batch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
    UseSimpleBatch: true,
})
batch.CreateFile("deep/path/file.txt", []byte("content"))
result, err := batch.Run() // Uses SimpleBatch implementation
```

### Advanced Configuration

```go
// Full control over pipeline options
batch := synthfs.NewBatch()
batch.CreateFile("deep/path/file.txt", []byte("content"))

opts := synthfs.PipelineOptions{
    Restorable:           true,
    MaxBackupSizeMB:      20,
    ResolvePrerequisites: true, // Enable prerequisite resolution
}
result, err := batch.RunWithOptions(opts)
```

## Migration Timeline

### Phase 5: Migration Path (Current)
- ✅ SimpleBatch implementation available
- ✅ `NewBatchWithOptions()` for gradual migration
- ✅ `RunWithPrerequisites()` methods
- ✅ `ResolvePrerequisites` flag in `PipelineOptions`

### Phase 6: Switch Defaults (Future)
- 🔄 `UseSimpleBatch` will default to `true`
- 🔄 Old batch methods will be deprecated
- 🔄 All internal usage will switch to new pattern

### Phase 7: Cleanup (Major Version)
- ⏳ Remove old batch implementation
- ⏳ Remove compatibility flags
- ⏳ Simplify codebase

## Recommended Migration Strategy

### For New Code
Use `NewSimpleBatch()` or `NewBatchWithOptions(synthfs.BatchOptions{UseSimpleBatch: true})`:

```go
batch := synthfs.NewSimpleBatch()
// ... add operations
result, err := batch.Run() // Prerequisite resolution enabled by default
```

### For Existing Code
Gradually migrate using opt-in methods:

```go
// Step 1: Test with prerequisite resolution
batch := synthfs.NewBatch()
// ... add operations
result, err := batch.RunWithPrerequisites() // Test new behavior

// Step 2: Once validated, switch to SimpleBatch
batch := synthfs.NewSimpleBatch()
// ... add operations  
result, err := batch.Run()
```

### For Libraries
Provide migration options to your users:

```go
type Config struct {
    UseSimpleBatch bool
}

func CreateBatch(cfg Config) *synthfs.Batch {
    return synthfs.NewBatchWithOptions(synthfs.BatchOptions{
        UseSimpleBatch: cfg.UseSimpleBatch,
    })
}
```

## Breaking Changes

### None in Phase 5
- All existing code continues to work unchanged
- New functionality is opt-in only
- Default behavior is preserved

### Future Breaking Changes (Phase 6+)
- Default will switch to `UseSimpleBatch: true`
- Deprecation warnings for old methods
- Eventually old implementation will be removed

## Benefits of Migration

### For Users
- **Better Error Messages**: Prerequisites provide clearer validation errors
- **More Extensible**: Add custom prerequisite types
- **Testable**: Mock prerequisite resolution

### For Maintainers
- **Less Hardcoded Logic**: No more operation type strings
- **Single Responsibility**: Each component has clear purpose
- **Easier to Add Features**: New operations just implement `Prerequisites()`

## Troubleshooting

### Prerequisites Not Resolved
```go
// Ensure prerequisite resolution is enabled
batch.RunWithPrerequisites()
// or
batch.RunWithOptions(synthfs.PipelineOptions{ResolvePrerequisites: true})
```

### Missing Parent Directories
```go
// Old behavior (hardcoded parent creation)
batch.Run()

// New behavior (prerequisite resolution)
batch.RunWithPrerequisites()
```

### Performance Differences
- Prerequisite resolution adds small overhead for validation
- Benefit: Better error messages and extensibility
- Cached validation results minimize impact

## Next Steps

1. **Try SimpleBatch**: Test `NewSimpleBatch()` in non-critical code
2. **Use RunWithPrerequisites**: Test prerequisite resolution with existing batches
3. **Monitor Behavior**: Ensure prerequisite resolution works as expected
4. **Plan Migration**: Identify when to switch defaults in your codebase
5. **Provide Feedback**: Report any issues or unexpected behavior

For questions or issues, please check the documentation or file an issue.