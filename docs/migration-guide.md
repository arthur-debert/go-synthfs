# Migration Guide: Execution Refactoring (Phase 4-6)

This document describes the migration path for the execution refactoring that introduces the SimpleBatch implementation and switches to prerequisite-driven operation handling.

## Overview of Changes

The execution refactoring introduces a new way of handling filesystem operations:

- **Old Approach**: Hardcoded parent directory creation in batch implementation
- **New Approach**: Operations declare prerequisites, execution pipeline resolves them

## Phase 4: SimpleBatch Alternative (DONE)

### What's New

- `SimpleBatch` implementation (`pkg/synthfs/batch/simple_batch.go`)
- No hardcoded operation type logic
- Relies on prerequisite resolution system
- Clean separation of concerns

### Usage

```go
// Create SimpleBatch directly
fs := filesystem.NewOSFileSystem(".")
registry := operations.NewFactory()
batch := batch.NewSimpleBatch(fs, registry)

// Create file - no auto-parent creation
_, err := batch.CreateFile("subdir/file.txt", []byte("content"))

// Run with prerequisite resolution
result, err := batch.RunWithPrerequisites()
```

## Phase 5: Migration Path (DONE)

### What's New

- `BatchOptions` struct for controlling behavior
- `NewBatchWithOptions()` factory method
- `UseSimpleBatch` flag (default: false in Phase 5)

### Migration Examples

```go
// Old way - still works
batch := synthfs.NewBatch()

// New way - explicit SimpleBatch
batch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
    UseSimpleBatch: true,
})

// New way - explicit legacy behavior  
batch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
    UseSimpleBatch: false,
})
```

## Phase 6: Switch Defaults (DONE)

### What Changed

- `NewBatch()` now defaults to `UseSimpleBatch: true`
- Added `NewBatchWithLegacyBehavior()` for temporary backward compatibility
- Deprecation notices on old methods

### Breaking Changes

**BEHAVIOR CHANGE**: `synthfs.NewBatch()` now uses SimpleBatch by default.

**Migration Required**: If you need the old behavior:

```go
// Option 1: Use legacy function (deprecated)
batch := synthfs.NewBatchWithLegacyBehavior()

// Option 2: Explicit options (recommended)
batch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
    UseSimpleBatch: false,
})
```

### Benefits of New Approach

1. **Extensibility**: New operation types just implement `Prerequisites()`
2. **Testability**: Each component has single responsibility
3. **Maintainability**: No hardcoded operation knowledge in batch
4. **Flexibility**: Operations can declare complex prerequisites

## Prerequisites System

### How It Works

Operations declare what they need:

```go
func (op *CreateFileOperation) Prerequisites() []core.Prerequisite {
    var prereqs []core.Prerequisite
    
    // Need parent directory to exist
    if filepath.Dir(op.path) != "." {
        prereqs = append(prereqs, core.NewParentDirPrerequisite(op.path))
    }
    
    // Need no conflict with existing files
    prereqs = append(prereqs, core.NewNoConflictPrerequisite(op.path))
    
    return prereqs
}
```

Execution pipeline resolves them:

```go
// SimpleBatch enables prerequisite resolution by default
result, err := batch.Run()

// Or explicitly
result, err := batch.RunWithPrerequisites()
```

### Available Prerequisites

- `core.ParentDirPrerequisite`: Parent directory must exist
- `core.NoConflictPrerequisite`: Path must not conflict with existing files
- `core.SourceExistsPrerequisite`: Source path must exist (for copy/move/delete)

## Common Migration Scenarios

### Scenario 1: Creating Nested Files

**Old code:**
```go
batch := synthfs.NewBatch()
batch.CreateFile("deep/nested/file.txt", content) // Auto-creates parents
```

**New code (Phase 6):**
```go
batch := synthfs.NewBatch() // Now uses SimpleBatch by default
batch.CreateFile("deep/nested/file.txt", content)
batch.Run() // Prerequisite resolution creates parents automatically
```

### Scenario 2: Complex Directory Structures

**Old code:**
```go
batch := synthfs.NewBatch()
batch.CreateFile("a/b/c/file1.txt", content1)
batch.CreateFile("a/b/d/file2.txt", content2)
// Parent directories created automatically
```

**New code:**
```go
batch := synthfs.NewBatch()
batch.CreateFile("a/b/c/file1.txt", content1)
batch.CreateFile("a/b/d/file2.txt", content2)
batch.Run() // Prerequisites resolved: creates a/, a/b/, a/b/c/, a/b/d/
```

### Scenario 3: Need Old Behavior

**Temporary workaround:**
```go
batch := synthfs.NewBatchWithLegacyBehavior() // DEPRECATED
// or
batch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
    UseSimpleBatch: false,
})
```

## Testing Your Migration

### Unit Tests

Verify your operations work with both implementations:

```go
func TestOperationWithBothBatches(t *testing.T) {
    // Test with SimpleBatch
    simpleBatch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
        UseSimpleBatch: true,
    })
    
    // Test with legacy batch
    legacyBatch := synthfs.NewBatchWithOptions(synthfs.BatchOptions{
        UseSimpleBatch: false,
    })
    
    // Both should produce same end result
}
```

### Integration Tests

Test prerequisite resolution:

```go
func TestPrerequisiteResolution(t *testing.T) {
    batch := synthfs.NewBatch() // Uses SimpleBatch
    batch.CreateFile("deep/path/file.txt", content)
    
    result, err := batch.Run()
    // Should succeed - prerequisites resolved automatically
}
```

## Troubleshooting

### "Path already exists" errors

**Problem**: Files exist but SimpleBatch can't detect them
**Solution**: Check filesystem interface implementation

### "Parent directory not found" errors  

**Problem**: Prerequisite resolution disabled
**Solution**: Use `RunWithPrerequisites()` or ensure `ResolvePrerequisites: true`

### Performance differences

**Problem**: Different execution order between old/new batch
**Solution**: Review operation dependencies and prerequisites

## Timeline

- **Phase 4 (DONE)**: SimpleBatch implementation available
- **Phase 5 (DONE)**: Migration path with `UseSimpleBatch: false` default
- **Phase 6 (DONE)**: Switch to `UseSimpleBatch: true` default, deprecation notices
- **Phase 7 (Future)**: Remove legacy implementation entirely

## Getting Help

If you encounter issues during migration:

1. Check that operations implement `Prerequisites()` correctly
2. Verify filesystem interface supports required operations
3. Test with both `UseSimpleBatch: true` and `UseSimpleBatch: false`
4. Review prerequisite resolution logs