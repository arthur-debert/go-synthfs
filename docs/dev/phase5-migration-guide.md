# Phase 5 Migration Guide: SimpleBatch Behavior

This guide explains how to migrate from the legacy batch behavior to the new SimpleBatch behavior that uses prerequisite resolution.

## Overview

The synthfs batch system now supports two modes:

1. **Legacy Mode** (default): Automatic parent directory creation with hardcoded logic
2. **SimpleBatch Mode**: Clean prerequisite resolution without hardcoded logic

## Migration Options

### Option 1: New Constructor (Recommended)

For new code, use the `NewBatchWithSimpleBatch()` constructor:

```go
// New way - recommended for new code
batch := synthfs.NewBatchWithSimpleBatch()
batch.CreateFile("path/to/file.txt", []byte("content"))
result, err := batch.Run() // Automatically resolves prerequisites
```

### Option 2: Enable Flag (For Existing Code)

For existing code that needs gradual migration:

```go
// Existing code
batch := synthfs.NewBatch()

// Enable SimpleBatch behavior
batch = batch.WithSimpleBatch(true)

batch.CreateFile("path/to/file.txt", []byte("content"))
result, err := batch.Run() // Now uses prerequisite resolution
```

### Option 3: Explicit Prerequisite Methods

Use explicit prerequisite resolution methods (works in both modes):

```go
batch := synthfs.NewBatch() // Legacy mode
batch.CreateFile("path/to/file.txt", []byte("content"))
result, err := batch.RunWithPrerequisites() // Forces prerequisite resolution
```

## Key Differences

### Legacy Mode Behavior
- Automatically creates parent directories during batch building
- Uses internal path state tracking for conflict detection
- Parent directories are created with hardcoded logic
- Operations have automatic dependencies added

### SimpleBatch Mode Behavior
- No automatic parent directory creation during batch building
- Relies on operation prerequisites (declared by operations themselves)
- Parent directories are created by prerequisite resolver during execution
- Clean separation of concerns

## Benefits of SimpleBatch Mode

1. **Cleaner Architecture**: Operations declare their own needs
2. **Extensibility**: New operation types work automatically 
3. **Testability**: Each component has single responsibility
4. **Predictability**: No hidden dependency injection
5. **Performance**: No overhead from path state tracking

## Migration Timeline

- **Phase 5** (Current): Both modes available, legacy is default
- **Phase 6** (Future): SimpleBatch becomes default  
- **Phase 7** (Future): Legacy mode removed

## Testing Your Migration

Both modes should produce the same results for basic operations:

```go
func TestMigration(t *testing.T) {
    // Legacy mode
    legacyBatch := synthfs.NewBatch()
    legacyBatch.CreateFile("dir/file.txt", []byte("content"))
    legacyResult, err := legacyBatch.Run()
    
    // SimpleBatch mode  
    simpleBatch := synthfs.NewBatchWithSimpleBatch()
    simpleBatch.CreateFile("dir/file.txt", []byte("content"))
    simpleResult, err := simpleBatch.Run()
    
    // Both should succeed and create the same files
    assert.True(t, legacyResult.Success)
    assert.True(t, simpleResult.Success)
}
```

## Troubleshooting

### "Parent directory does not exist" errors

If you get prerequisite validation errors in SimpleBatch mode, make sure you're calling the correct run method:

```go
// Wrong - might fail if prerequisites aren't resolved
result, err := batch.RunWithOptions(synthfs.PipelineOptions{
    ResolvePrerequisites: false, // This disables prerequisite resolution
})

// Right - prerequisites are resolved automatically  
result, err := batch.Run() // SimpleBatch enables prerequisites by default
```

### Performance Differences

SimpleBatch mode may be slightly faster due to:
- No path state tracking overhead
- No conflict checking during batch building
- More efficient prerequisite resolution

## Backwards Compatibility

Legacy mode will continue to work exactly as before until Phase 7. No existing code needs to change immediately.