# REFAC:MISSING-BITS-ERROR-HANDLING Align Error Handling with Original Batch Implementation

## Issue Summary

The `TestSimpleBatchWithRollback` test is currently skipped because error types returned by Simple API don't match the original batch implementation expectations. The test expects `PipelineError` but receives `*core.RollbackError`.

## Current Status

**Skipped Test**: `TestSimpleBatchWithRollback` in `pkg/synthfs/simple_batch_test.go`

**Test Failure**:
```
simple_batch_test.go:366: Expected PipelineError, got *core.RollbackError
```

## Root Cause Analysis

When the batch API was removed, the Simple API started using the execution package's error handling directly, but the execution package returns different error types than the original batch API did.

### Original vs Current Error Flow

**Original (Batch API)**:
```
Operation fails → Batch wraps in PipelineError → Test expects PipelineError ✅
```

**Current (Simple API)**:  
```
Operation fails → Execution package returns *core.RollbackError → Test expects PipelineError ❌
```

## Required Work

### 1. Study Original Error Handling

**Reference**: `git show main:pkg/synthfs/batch/batch.go` - error handling in `Run()` and `RunWithOptions()`

The original batch implementation:
1. Caught errors from execution package
2. Wrapped them in appropriate error types (`PipelineError`, etc.)
3. Preserved error context and metadata
4. Returned consistent error interface to users

### 2. Update Simple API Error Handling

**Files to modify**:
- `pkg/synthfs/simple_api.go` - `Run()` and `RunWithOptions()` methods

**Current behavior**:
```go
func RunWithOptions(ctx context.Context, fs filesystem.FileSystem, options PipelineOptions, ops ...Operation) (*Result, error) {
    // ... execution logic ...
    result := executor.RunWithOptions(ctx, prevalidatedPipeline, fs, options)
    
    // Return raw result and error from execution package
    var err error
    if !result.Success {
        if len(result.Errors) > 0 {
            err = result.Errors[0]  // Raw error from execution package
        }
    }
    return result, err
}
```

**Target behavior**:
```go
func RunWithOptions(...) (*Result, error) {
    // ... execution logic ...
    result := executor.RunWithOptions(ctx, prevalidatedPipeline, fs, options)
    
    // Wrap errors in appropriate types to match original batch behavior
    var err error
    if !result.Success {
        if len(result.Errors) > 0 {
            err = wrapExecutionError(result.Errors[0], result, ops)  // Wrapped error
        }
    }
    return result, err
}
```

### 3. Implement Error Wrapping Logic

**Create error wrapping functions**:
- `wrapExecutionError()` - Convert execution package errors to batch-compatible errors
- Preserve error messages, stack traces, and context
- Map `*core.RollbackError` → `PipelineError` where appropriate
- Handle other execution error types consistently

### 4. Define Error Type Mapping

Map execution package error types to expected batch error types:

| Execution Error | Expected Batch Error | Context |
|----------------|---------------------|---------|
| `*core.RollbackError` | `PipelineError` | Operation rollback scenarios |
| `*core.ValidationError` | `ValidationError` | Operation validation failures |
| `*core.ExecutionError` | `PipelineError` | General execution failures |

### 5. Preserve Error Information

Ensure wrapped errors maintain:
- **Original error message**
- **Stack trace information** 
- **Failed operation details**
- **Execution context** (operation index, successful operations, etc.)

## Implementation Strategy

### Phase 1: Study Original Error Handling
```bash
git show main:pkg/synthfs/batch/batch.go | grep -A 20 -B 5 "Error\|err"
# Study how batch implementation wrapped and returned errors
```

### Phase 2: Identify Error Mapping
1. Run failing test to see exact error types and messages
2. Compare with expected error types in test assertions
3. Create mapping from execution errors to batch errors

### Phase 3: Implement Error Wrapping
1. Create helper functions for error wrapping
2. Update `RunWithOptions()` to use error wrapping
3. Preserve all error context and information

### Phase 4: Test and Verify
1. Run `TestSimpleBatchWithRollback` to verify error handling
2. Remove `t.Skip()` from the test
3. Ensure all error assertions pass
4. Verify error messages and context are preserved

## Success Criteria

✅ `TestSimpleBatchWithRollback` passes without being skipped  
✅ `PipelineError` returned when expected by tests  
✅ Error messages match original batch implementation format  
✅ Error context preserved (failed operation details, etc.)  
✅ No regression in other error handling scenarios  
✅ Error types consistent across all Simple API operations  

## Files to Modify

**Primary**:
- `pkg/synthfs/simple_api.go` - Update error handling in `Run()` and `RunWithOptions()`

**Supporting**:
- `pkg/synthfs/errors.go` (if exists) - Error wrapping utilities
- `pkg/synthfs/types.go` - Error type definitions (if needed)

**Test**:
- `pkg/synthfs/simple_batch_test.go` - Remove `t.Skip()` line

**Reference**:
- Original batch error handling on main branch

## Priority

**Medium** - Error handling is important for user experience, but doesn't affect core functionality. The operations work correctly; only the error types are mismatched.