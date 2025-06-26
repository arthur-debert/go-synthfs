# Code Mapping for SynthFS Restructuring

This document maps existing code to the new package structure as outlined in `README.txt`.

## Core Types and Interfaces

### Current Location -> New Location

- `pkg/synthfs/fs.go` -> `pkg/synthfs/types.go`
  - `ReadFS` interface
  - `WriteFS` interface
  - `FileSystem` interface
  - `StatFS` interface
  - `FullFileSystem` interface

- `pkg/synthfs/operation.go` -> `pkg/synthfs/types.go`
  - `Operation` interface
  - `OperationID` type
  - `OperationDesc` struct
  - `BackupData` struct
  - `BackedUpItem` struct
  - `BackupBudget` struct

- `pkg/synthfs/items.go` -> `pkg/synthfs/types.go`
  - `FsItem` interface

## Constants

### Current Location -> New Location

- `pkg/synthfs/items.go` -> `pkg/synthfs/constants.go`
  - `ArchiveFormat` constants

- `pkg/synthfs/state.go` -> `pkg/synthfs/constants.go`
  - Path state constants

## Errors

### Current Location -> New Location

- `pkg/synthfs/operation.go` -> `pkg/synthfs/errors.go`
  - `ValidationError`
  - `DependencyError`
  - `ConflictError`

## Target Types

### Current Location -> New Location

- `pkg/synthfs/items.go` -> `pkg/synthfs/targets/file.go`
  - `FileItem` struct
  - `NewFile` function

- `pkg/synthfs/items.go` -> `pkg/synthfs/targets/directory.go`
  - `DirectoryItem` struct
  - `NewDirectory` function

- `pkg/synthfs/items.go` -> `pkg/synthfs/targets/symlink.go`
  - `SymlinkItem` struct
  - `NewSymlink` function

- `pkg/synthfs/items.go` -> `pkg/synthfs/targets/archive.go`
  - `ArchiveItem` struct
  - `NewArchive` function
  - `UnarchiveItem` struct
  - `NewUnarchive` function

## Operations

### Current Location -> New Location

- `pkg/synthfs/operation.go` -> `pkg/synthfs/operations/create.go`
  - File creation operations and validation
  - Directory creation operations and validation
  - Symlink creation operations and validation

- `pkg/synthfs/operation.go` -> `pkg/synthfs/operations/copy.go`
  - Copy operations and validation

- `pkg/synthfs/operation.go` -> `pkg/synthfs/operations/move.go`
  - Move operations and validation

- `pkg/synthfs/operation.go` -> `pkg/synthfs/operations/delete.go`
  - Delete operations and validation

- `pkg/synthfs/operation.go` -> `pkg/synthfs/operations/archive.go`
  - Archive creation operations and validation
  - Unarchive operations and validation

## Execution

### Current Location -> New Location

- `pkg/synthfs/pipeline.go` -> `pkg/synthfs/execution/pipeline.go`
  - `Pipeline` interface
  - `PipelineOptions` struct
  - `SimplePipeline` struct

- `pkg/synthfs/executor.go` -> `pkg/synthfs/execution/executor.go`
  - `Executor` struct
  - `NewExecutor` function
  - `Run` method

- `pkg/synthfs/batch.go` -> `pkg/synthfs/execution/batch.go`
  - `Batch` struct
  - Batch API methods

- `pkg/synthfs/state.go` -> `pkg/synthfs/execution/state.go`
  - `PathStateTracker` struct
  - State tracking methods

## Backup

### Current Location -> New Location

- `pkg/synthfs/operation.go` (relevant parts) -> `pkg/synthfs/backup/backup.go`
  - Backup methods from operations
  - `BackupData` handling

- `pkg/synthfs/operation.go` (relevant parts) -> `pkg/synthfs/backup/restore.go`
  - Restore/rollback operations
  - `ReverseOps` methods

## Filesystem

### Current Location -> New Location

- `pkg/synthfs/fs.go` -> `pkg/synthfs/filesystem/interfaces.go`
  - All filesystem interfaces

- `pkg/synthfs/fs.go` -> `pkg/synthfs/filesystem/os.go`
  - `OSFileSystem` implementation

- `pkg/synthfs/testing.go` -> `pkg/synthfs/filesystem/memory.go`
  - `TestFileSystem` implementation

## Validation

### Current Location -> New Location

- `pkg/synthfs/operation.go` (checksum parts) -> `pkg/synthfs/validation/checksum.go`
  - `ChecksumRecord` struct
  - Checksum computation and verification

- `pkg/synthfs/operation.go` (validation parts) -> `pkg/synthfs/validation/validator.go`
  - Validation utilities
  - Validation logic common across operations

## Root Package

### Remain at Root

- `pkg/synthfs/log.go` -> stays at root
- `pkg/synthfs/testing.go` (non-filesystem parts) -> stays at root
