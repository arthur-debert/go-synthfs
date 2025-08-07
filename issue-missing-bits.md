# REFAC:MISSING-BITS Remove Adapters in API

## Issue Summary

Phase 4 removed the batch API but left the adapter layer (`pipelineWrapper`, `operationInterfaceAdapter`) as a bridge to the execution system. These adapters are broken and causing test failures. The adapters need to be removed and functionality implemented directly in the Simple API.

## Root Cause Analysis

The current architecture has a fundamental mismatch:
```
Simple API Operation → operationInterfaceAdapter → execution system → Result
     ↑                           ↑                      ↑             ↑
   Missing           Broken interface        Different        Wrong type
   features         implementation        error handling      structure
```

### Current Test Failures

1. **Interface Conversion Panic**: `panic: interface conversion: *synthfs.operationInterfaceAdapter is not operations.Operation`
2. **Missing Checksum Functionality**: `Expected checksum to be computed for source file` (0 checksums found)
3. **Error Type Mismatch**: `Expected PipelineError, got *core.RollbackError`
4. **Operation Output Issues**: Type assertions failing for operation output capture

## Required Work

### 1. Core Execution Replacement
**Current**: Simple API uses adapters to bridge to `execution` package  
**Target**: Direct execution in Simple API without adapters

**Changes Needed**:
- Replace `executor.RunWithOptions(ctx, wrapper, fs, opts)` in `simple_api.go`
- Implement direct operation execution loop (port from `batch/batch.go`)
- Handle pipeline options (DryRun, RollbackOnError, etc.) directly
- Generate proper Result objects without adapter conversion

### 2. Restore Missing Functionality
Port logic from original batch implementation (`git show main:pkg/synthfs/batch/batch.go`)

#### A. Checksum Computation (Critical)
- **Original**: Batch operations automatically computed checksums for Copy/Move/Archive
- **Current**: Simple API operations don't compute checksums
- **Fix**: Port checksum logic from batch operation creation methods

#### B. Metadata Support
- **Original**: All batch operations supported `metadata ...map[string]interface{}`
- **Current**: Simple API operations lack metadata parameters
- **Fix**: Add metadata support to Simple API operation methods in `synthfs.go`

#### C. Operation Output Capture
- **Original**: Operations supported output capture through proper interfaces
- **Current**: Type assertion fails on adapter references
- **Fix**: Ensure Simple API operations directly support output capture methods

### 3. Error Handling Alignment
- Return `PipelineError` instead of `core.RollbackError` where expected
- Preserve error message formats and error wrapping behavior  
- Handle rollback errors same as original implementation

### 4. Operation Interface Consistency
- Store actual operation references in `Result.Operations` instead of adapter references
- Ensure operations implement expected interfaces for output capture
- Remove all adapter type assertions from test code

## Implementation Strategy

### Phase 1: Port Execution Logic
1. **Study** `git show main:pkg/synthfs/batch/batch.go` execution patterns
2. **Replace** adapter-based execution in `simple_api.go` with direct logic
3. **Port** operation validation, execution loop, and result generation
4. **Remove** `pipelineWrapper` and `operationInterfaceAdapter` from `executor.go`

### Phase 2: Restore Features
1. **Add metadata parameters** to all operation creation methods in `synthfs.go`
2. **Implement checksum computation** during Copy/Move/Archive operation creation
3. **Ensure output capture** works on actual operations, not adapters
4. **Fix error types** to match original batch behavior

### Phase 3: Verify and Clean
1. **Run full test suite** to ensure no functionality lost
2. **Remove any remaining adapter references** from tests
3. **Verify** all original batch features work through Simple API
4. **Document** the unified Simple API as sole interface

## Files to Modify

### Primary Implementation
- **`simple_api.go`**: Replace execution logic, remove adapter dependency
- **`synthfs.go`**: Add metadata parameters to operation creation
- **`executor.go`**: Remove adapter classes or refactor to work directly

### Reference Files (for behavior)
- **`batch/batch.go`** (on main): Execution logic, checksum computation
- **`batch/factory.go`** (on main): Operation creation with metadata
- **Batch tests** (on main): Expected behavior patterns

### Test Files (verification)
- **`validation/checksum_test.go`**: Verify checksum functionality restored
- **`output_capture_test.go`**: Verify operation output capture works
- **`simple_batch_test.go`**: Verify error handling alignment

## Success Criteria

✅ All tests compile and pass  
✅ No adapter classes remain in codebase  
✅ Simple API operations support metadata parameters  
✅ Checksum computation works for Copy/Move/Archive operations  
✅ Operation output capture works without type assertion failures  
✅ Error types match original batch API behavior  
✅ Simple API is sole external interface with full feature parity  

## Risk Assessment

**Low Risk**: This is primarily a logic porting exercise - the behavior patterns, error handling, and feature set already exist in the original batch implementation on main branch.

**Mitigation**: Study original implementation carefully and port incrementally with continuous testing to ensure no functionality is lost.