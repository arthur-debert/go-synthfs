# REFAC:MISSING-BITS-CHECKSUM Restore Checksum Functionality in Simple API

## Issue Summary

The `TestBatchChecksumming` test is currently skipped because checksum computation functionality was lost when converting from batch API to Simple API. The Simple API operations (Copy, Move, Archive) need to compute and store checksums like the original batch implementation.

## Current Status

**Skipped Test**: `TestBatchChecksumming` in `pkg/synthfs/validation/checksum_test.go`

**Test Failures**:
```
checksum_test.go:38: Expected checksum to be computed for source file
checksum_test.go:150: Expected 3 checksums, got 0
checksum_test.go:156: Expected checksum for source file file1.txt
```

## Root Cause Analysis

The original batch API automatically computed checksums during operation creation, but when operations were converted to Simple API, this functionality was not ported over.

### Original Implementation (on main branch)

In the batch implementation, operations like Copy/Move/Archive would:
1. Compute MD5 checksums of source files during operation creation
2. Store checksums in operation metadata via `SetChecksum(path, checksum)`
3. Add checksum details to operation descriptions
4. Make checksums available via `GetChecksum(path)` and `GetAllChecksums()` methods

## Required Work

### 1. Port Checksum Logic from Batch Implementation

**Reference**: `git show main:pkg/synthfs/batch/batch.go` (around line 200-300)

The original batch implementation computed checksums in methods like:
- `Copy(src, dst string, metadata...)` - computed checksum for source file
- `Move(src, dst string, metadata...)` - computed checksum for source file  
- `CreateArchive(path, format, sources, metadata...)` - computed checksums for all source files

### 2. Update Simple API Operation Creation

**Files to modify**:
- `pkg/synthfs/synthfs.go` - Add checksum computation to Copy/Move/Archive methods

**Required changes**:
```go
// Current (no checksums)
func (s *SynthFS) Copy(src, dst string) Operation {
    // Just creates operation
}

// Target (with checksums)
func (s *SynthFS) Copy(src, dst string) Operation {
    // 1. Compute checksum of source file
    // 2. Store checksum in operation via SetChecksum()
    // 3. Add checksum to operation description details
}
```

### 3. Ensure Operation Interface Support

**Verify operations implement**:
- `GetChecksum(path string) interface{}`
- `GetAllChecksums() map[string]interface{}`
- `SetChecksum(path string, checksum interface{})` 

### 4. Checksum Computation Implementation

**Logic to port**:
1. **File Reading**: Read source file contents
2. **MD5 Computation**: Compute MD5 hash of contents
3. **ChecksumRecord Creation**: Create `validation.ChecksumRecord` with path, MD5, size
4. **Storage**: Store via `SetChecksum(path, checksumRecord)`
5. **Description Update**: Add `source_checksum` to operation details

### 5. Handle Different Operation Types

#### Copy Operations
- Compute checksum for source file only
- Store as `source_checksum` in operation details

#### Move Operations  
- Compute checksum for source file only
- Store as `source_checksum` in operation details

#### Archive Operations
- Compute checksums for ALL source files
- Store individual checksums via `GetChecksum(path)`
- Store count as `sources_checksummed` in operation details

## Implementation Strategy

### Phase 1: Study Original Implementation
```bash
git show main:pkg/synthfs/batch/batch.go > original-batch-implementation.txt
# Study checksum computation patterns
```

### Phase 2: Add Checksum Computation Helper
Create a helper function for computing file checksums that can be reused across operations.

### Phase 3: Update Operation Creation Methods
Update `Copy`, `Move`, and `CreateArchive` methods in `synthfs.go` to compute checksums during operation creation.

### Phase 4: Verify and Unskip Test
1. Run `TestBatchChecksumming` to verify all checksum functionality works
2. Remove `t.Skip()` from the test
3. Ensure all checksum-related assertions pass

## Success Criteria

✅ `TestBatchChecksumming` passes without being skipped  
✅ Copy operations compute source file checksums  
✅ Move operations compute source file checksums  
✅ Archive operations compute checksums for all source files  
✅ Checksums accessible via `GetChecksum(path)` and `GetAllChecksums()`  
✅ Operation descriptions include checksum information  
✅ ChecksumRecord objects contain correct path, MD5, and size  

## Files to Modify

**Primary**:
- `pkg/synthfs/synthfs.go` - Add checksum computation to operation creation methods

**Test**:
- `pkg/synthfs/validation/checksum_test.go` - Remove `t.Skip()` line

**Reference** (for porting logic):
- Original batch implementation on main branch

## Priority

**High** - Checksums are a core security/integrity feature that was working in the original implementation.