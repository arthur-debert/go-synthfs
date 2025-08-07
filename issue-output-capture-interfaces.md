# REFAC:MISSING-BITS-OUTPUT-CAPTURE Fix Operation Output Capture Interface Issues

## Issue Summary

Three output capture tests are currently skipped due to interface conversion panics when trying to access operation output methods. The problem is that operations in results are wrapped in `operationInterfaceAdapter` objects that don't properly implement the expected interfaces.

## Current Status

**Skipped Tests**:
- `TestShellCommand_OutputCapture` 
- `TestCustomOperation_OutputCapture`
- `TestOutputCapture_RealWorldExample`

**Test Failure**:
```
panic: interface conversion: *synthfs.operationInterfaceAdapter is not operations.Operation: missing method Execute
```

**Error Location**: `output_capture_test.go:45` when calling:
```go
stdout := synthfs.GetOperationOutput(opResult.Operation.(synthfs.Operation), "stdout")
```

## Root Cause Analysis

The issue occurs because:

1. **Result.Operations contains adapter references**: When operations are executed, the results contain `operationInterfaceAdapter` objects, not the actual operations
2. **Broken adapter implementation**: The `operationInterfaceAdapter` claims to implement interfaces but is missing required methods
3. **Type assertion fails**: Tests try to cast `opResult.Operation` to `synthfs.Operation` but the runtime type is incompatible

### Current Flow (Broken)
```
Simple API Operation → operationInterfaceAdapter → Result.Operations → Type assertion fails ❌
```

### Target Flow
```
Simple API Operation → Direct operation reference → Result.Operations → Type assertion succeeds ✅
```

## Required Work

### 1. Fix Operation References in Results

**Problem**: `Result.Operations[].Operation` contains adapter objects instead of actual operations

**Solution**: Ensure actual operation objects are stored in results, not adapters

**Files to modify**:
- `pkg/synthfs/executor.go` - Remove adapter wrapping in result conversion
- `pkg/synthfs/simple_api.go` - Ensure operations stored directly in results

### 2. Remove Adapter Dependencies

**Current problematic code** (in `executor.go`):
```go
func (pw *pipelineWrapper) Operations() []interface{} {
    ops := pw.pipeline.Operations()
    var result []interface{}
    for _, op := range ops {
        result = append(result, &operationInterfaceAdapter{Operation: op})  // ❌ Wrapping in adapter
    }
    return result
}
```

**Target**:
```go
func (pw *pipelineWrapper) Operations() []interface{} {
    ops := pw.pipeline.Operations()
    var result []interface{}
    for _, op := range ops {
        result = append(result, op)  // ✅ Direct operation reference
    }
    return result
}
```

### 3. Ensure Operation Interface Compliance

**Verify that Simple API operations implement required methods**:
- `GetOperationOutput(operation, key) string` - for stdout/stderr capture
- `GetOperationOutputValue(operation, key) interface{}` - for structured output
- `GetAllOperationOutputs(operation) map[string]string` - for complete output

**Check operation implementations in**:
- `pkg/synthfs/operations/` - Core operation implementations
- `pkg/synthfs/synthfs.go` - Simple API operation creation

### 4. Fix Output Capture Method Implementations

**Current issue**: Operations may not have proper output capture method implementations

**Solution**: Ensure operations support output capture methods either:
- **Directly**: Operations implement output methods themselves
- **Via interface**: Operations implement a common output interface
- **Via helper**: Output methods work on any operation type

### 5. Update Test Assertions

**Remove problematic type assertions**:
```go
// Current (fails)
stdout := synthfs.GetOperationOutput(opResult.Operation.(synthfs.Operation), "stdout")

// Target (works)  
stdout := synthfs.GetOperationOutput(opResult.Operation, "stdout")
```

Or ensure the type assertion works by fixing the underlying operation references.

## Implementation Strategy

### Phase 1: Diagnose Adapter Issue
1. **Run failing test with debug**: Understand exact runtime types in `opResult.Operation`
2. **Trace operation flow**: Follow how operations get wrapped in adapters
3. **Identify wrapping location**: Find where adapters are introduced

### Phase 2: Fix Operation References
1. **Remove adapter wrapping** in result generation
2. **Store direct operation references** in `Result.Operations`
3. **Test type assertions** work correctly

### Phase 3: Verify Interface Implementations
1. **Check operation method implementations**: Ensure operations support output capture
2. **Fix missing methods** if operations don't implement required interfaces
3. **Test output capture methods** work on actual operations

### Phase 4: Unskip and Verify Tests
1. **Remove `t.Skip()` from all three tests**
2. **Run tests individually** to verify each passes
3. **Run all output capture tests together** to verify no interactions

## Success Criteria

✅ `TestShellCommand_OutputCapture` passes without being skipped  
✅ `TestCustomOperation_OutputCapture` passes without being skipped  
✅ `TestOutputCapture_RealWorldExample` passes without being skipped  
✅ `opResult.Operation.(synthfs.Operation)` type assertions succeed  
✅ `GetOperationOutput()` methods work correctly  
✅ `GetOperationOutputValue()` methods work correctly  
✅ `GetAllOperationOutputs()` methods work correctly  
✅ No panics or interface conversion errors  

## Files to Modify

**Primary**:
- `pkg/synthfs/executor.go` - Remove adapter wrapping in results
- `pkg/synthfs/simple_api.go` - Ensure direct operation references

**Supporting**:
- `pkg/synthfs/operations/*.go` - Verify operation interface implementations
- `pkg/synthfs/synthfs.go` - Check operation creation methods

**Test**:
- `pkg/synthfs/output_capture_test.go` - Remove three `t.Skip()` lines

## Dependencies

This issue is closely related to **REFAC:MISSING-BITS** (adapter removal). The output capture issues will likely be resolved when adapters are completely removed from the system.

## Priority

**Medium** - Output capture is an advanced feature used for shell commands and custom operations. Core filesystem operations work correctly without this functionality.