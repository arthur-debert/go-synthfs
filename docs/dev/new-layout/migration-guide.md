# Migration Guide: Moving to Package-Based Architecture

## Overview

This guide explains how to migrate from the monolithic design to the new package-based architecture where operations and batch are independent packages.

## Current State

We have implemented:
1. **Core package** with shared types and interfaces
2. **Operations package** with interface{}-based design
3. **Batch package** with clean implementation
4. **Adapters** for backward compatibility during migration

## Migration Steps

### Step 1: Update Operation Implementations

For each operation type (create, copy, move, delete, archive), migrate from the main package to the operations package.

#### Example: Migrating CreateFileOperation

**Old (main package):**
```go
// In operation_simple.go
func (op *SimpleOperation) executeCreateFile(ctx context.Context, fsys FileSystem) error {
    fileItem, ok := op.item.(*FileItem)  // Coupled to FileItem type
    // ...
}
```

**New (operations package):**
```go
// In operations/create.go
func (op *CreateFileOperation) Execute(ctx context.Context, fsys interface{}) error {
    item := op.GetItem()  // Returns interface{}
    // Use interface assertions to work with the item
}
```

### Step 2: Update Operation Factory

The operation factory needs to create operations from the operations package instead of main package.

**Current:**
```go
func (r *DefaultRegistry) CreateOperation(id OperationID, opType string, path string) (interface{}, error) {
    return NewSimpleOperation(id, opType, path), nil
}
```

**Target:**
```go
func (r *DefaultRegistry) CreateOperation(id OperationID, opType string, path string) (interface{}, error) {
    switch opType {
    case "create_file":
        return operations.NewCreateFileOperation(id, path), nil
    case "create_directory":
        return operations.NewCreateDirectoryOperation(id, path), nil
    // ... other operation types
    }
}
```

### Step 3: Update Batch to Use New Operations

The batch should work entirely with interface{} operations from the operations package.

**Current flow:**
1. Batch creates SimpleOperation (main package)
2. Sets properties on the operation
3. Validates and executes

**New flow:**
1. Batch creates operation via factory (returns operations package type)
2. Sets properties through interfaces
3. Validates and executes through interfaces

### Step 4: Update Tests

Tests need to be updated to work with the new package structure:

```go
// Old test
func TestCreateFile(t *testing.T) {
    op := NewSimpleOperation("test", "create_file", "/test.txt")
    fileItem := NewFile("/test.txt").WithContent([]byte("test"))
    op.SetItem(fileItem)
    // ...
}

// New test
func TestCreateFile(t *testing.T) {
    op := operations.NewCreateFileOperation("test", "/test.txt")
    op.SetDescriptionDetail("content", []byte("test"))
    op.SetDescriptionDetail("mode", fs.FileMode(0644))
    // ...
}
```

### Step 5: Remove Adapters

Once all code is migrated:
1. Remove `adapters.go`
2. Remove `batch_adapter.go`
3. Remove old operation implementations from main package
4. Update all imports

## Key Patterns

### Working with Interface{} Types

Since operations return `interface{}` instead of concrete types, use these patterns:

```go
// Getting items
item := op.GetItem()
if fileItem, ok := item.(interface{ Path() string }); ok {
    path := fileItem.Path()
}

// Getting checksums
checksum := op.GetChecksum(path)
if cs, ok := checksum.(interface{ GetMD5() string }); ok {
    md5 := cs.GetMD5()
}
```

### Interface Assertions for Filesystem

```go
// Check if filesystem supports WriteFile
type writeFS interface {
    WriteFile(name string, data []byte, perm interface{}) error
}

if fs, ok := fsys.(writeFS); ok {
    err := fs.WriteFile(path, data, perm)
}
```

## Benefits After Migration

1. **No Circular Dependencies**: Operations and batch are independent packages
2. **Clean Architecture**: Each package has clear responsibilities
3. **Extensibility**: Easy to add new operation types
4. **Testability**: Packages can be tested in isolation
5. **No More Adapters**: Direct use of package types

## Timeline

1. **Phase 1** (Current): Basic structure and adapters in place
2. **Phase 2**: Migrate all operation types to operations package
3. **Phase 3**: Update batch to work directly with operations package
4. **Phase 4**: Remove adapters and old code
5. **Phase 5**: Update documentation and examples

## Common Issues and Solutions

### Issue: Type assertion failures
**Solution**: Ensure you're checking for the correct interface, not concrete type

### Issue: Missing methods after migration
**Solution**: Add the methods to the interface or use a more specific interface assertion

### Issue: Tests failing after migration
**Solution**: Update tests to use the new interface-based approach

## Checklist

- [ ] All operation types migrated to operations package
- [ ] Batch package fully implemented
- [ ] Factory creates operations from operations package
- [ ] All tests updated and passing
- [ ] Adapters removed
- [ ] Old code deleted from main package
- [ ] Documentation updated
- [ ] Examples updated