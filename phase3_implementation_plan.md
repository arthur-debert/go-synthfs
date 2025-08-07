# Phase 3: Triple Adapter Elimination - Implementation Plan

## Executive Summary

This plan outlines the steps to eliminate three adapter patterns in the codebase:
- **OperationsPackageAdapter** (135 lines)
- **CustomOperationAdapter** (122 lines) 
- **operationWrapper** (~110 lines)

Total code reduction: ~367 lines

## Prerequisites

Before starting Phase 3, ensure:
1. All tests are passing
2. No pending changes in working directory
3. Create a feature branch: `refac/phase3-adapter-elimination`

## Implementation Steps

### Step 1: Interface Consolidation (Breaking Change)

**Goal**: Create a unified Operation interface that both packages can implement.

**Changes**:
1. Create `core/operation_interface.go`:
```go
package core

// Operation is the unified interface for all operations
type Operation interface {
    // Core metadata
    OperationMetadata
    
    // Execution with context awareness
    Executable
    
    // Prerequisites
    Prerequisites() []Prerequisite
    
    // Operation-specific methods
    Rollback(ctx interface{}, fsys interface{}) error
    GetItem() interface{}
    GetChecksum(path string) interface{}
    GetAllChecksums() map[string]interface{}
    ReverseOps(ctx interface{}, fsys interface{}, budget *BackupBudget) ([]interface{}, *BackupData, error)
    
    // Mutation methods for batch operations
    SetDescriptionDetail(key string, value interface{})
    AddDependency(depID OperationID)
    SetPaths(src, dst string)
    SetChecksum(path string, checksum interface{})
}
```

2. Update `synthfs/types.go` to use core.Operation:
```go
type Operation = core.Operation
```

### Step 2: Update Operations Package

**Goal**: Make operations in the operations package implement the full interface directly.

**Changes**:
1. Update `operations/interfaces.go` to extend core.Operation
2. Modify `operations/base.go` to implement missing methods:
   - Add `SetChecksum` implementation
   - Ensure Execute/Validate match core.Executable signatures
3. Update return types in ReverseOps to return `[]interface{}` instead of `[]Operation`

### Step 3: Eliminate OperationsPackageAdapter

**Goal**: Remove the adapter and use operations directly.

**Changes**:
1. Update `synthfs.go` factory methods to return operations directly:
```go
func (s *SynthFS) CreateFile(path string, content []byte, mode fs.FileMode) Operation {
    id := s.idGen("create_file", path)
    op := operations.NewCreateFileOperation(id, path)
    item := targets.NewFile(path).WithContent(content).WithMode(mode)
    op.SetItem(item)
    return op // No adapter needed
}
```

2. Update `registry.go`:
```go
func (r *OperationRegistry) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
    return r.operationsFactory.CreateOperation(id, opType, path)
}
```

3. Delete `operations_adapter.go`

### Step 4: Update CustomOperation

**Goal**: Make CustomOperation implement the Operation interface directly.

**Changes**:
1. Update `custom_operation.go` to implement missing methods:
```go
func (op *CustomOperation) GetItem() interface{} {
    return nil // Custom operations don't have filesystem items
}

func (op *CustomOperation) GetChecksum(path string) interface{} {
    return nil // Custom operations don't manage checksums
}

func (op *CustomOperation) GetAllChecksums() map[string]interface{} {
    return nil
}

func (op *CustomOperation) SetChecksum(path string, checksum interface{}) {
    // No-op for custom operations
}
```

### Step 5: Eliminate CustomOperationAdapter

**Goal**: Remove the adapter and use CustomOperation directly.

**Changes**:
1. Update all factory methods in `synthfs.go` to return CustomOperation directly
2. Update `shell_command.go` to return CustomOperation directly
3. Delete `custom_operation_adapter.go`

### Step 6: Update Execution Package Interface

**Goal**: Make the execution package work with the standard Operation interface.

**Changes**:
1. Update `execution/interfaces.go` to use `core.Operation`
2. Modify pipeline wrapper to pass operations directly without wrapping

### Step 7: Eliminate operationWrapper

**Goal**: Remove the wrapper from executor.

**Changes**:
1. Update `executor.go`:
   - Remove operationWrapper struct
   - Update pipelineWrapper.Operations() to return operations directly
   - Update result conversion to handle operations directly
2. Remove GetSrcPath/GetDstPath methods (use operation methods directly)

### Step 8: Testing and Validation

**Goal**: Ensure no regressions.

**Actions**:
1. Run all tests: `./scripts/test`
2. Check for any type assertion failures
3. Verify executor still works correctly
4. Test batch operations
5. Test custom operations and shell commands

## Migration Guide for Users

### Breaking Changes
1. Operations now return `interface{}` for some methods instead of concrete types
2. Custom operations must implement additional methods
3. Type assertions may need updating

### Migration Steps
For users with custom operations:
```go
// Before
type MyOperation struct {
    synthfs.SimpleOperation
}

// After
type MyOperation struct {
    synthfs.SimpleOperation
}

// Add these methods:
func (op *MyOperation) GetItem() interface{} { return nil }
func (op *MyOperation) GetChecksum(path string) interface{} { return nil }
func (op *MyOperation) GetAllChecksums() map[string]interface{} { return nil }
func (op *MyOperation) SetChecksum(path string, checksum interface{}) {}
```

## Rollback Plan

If issues arise:
1. Git revert the commits in reverse order
2. Restore the adapter files
3. Re-run tests to ensure stability

## Success Criteria

1. All tests passing
2. No adapter files remaining
3. Simplified dependency graph
4. Performance benchmarks show improvement (less indirection)
5. Code coverage maintained or improved

## Timeline

Estimated time: 4-6 hours
- Step 1-2: 1 hour (interface updates)
- Step 3-5: 2 hours (adapter removal)
- Step 6-7: 1 hour (executor updates)
- Step 8: 1-2 hours (testing and fixes)

## Post-Implementation

1. Update documentation
2. Create migration guide
3. Tag as v0.6.0 (breaking change)
4. Update CHANGELOG.md