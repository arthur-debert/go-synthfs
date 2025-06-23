# Operations Implementation Plan

This document outlines the implementation plan for expanding synthfs to support creation, deletion, copying, and moving of files, directories, symlinks, and archives.

## How we work

We are currently in the design phase, reviewing and improving the design before implementation.
Once in execution stage, you will given a phase and steps to implement. Each phase is to be done
on a git branch. You can create the commits and push them as you go along, making sure to have target commits for each step (when possible), this would allow you to prefix your commit messages with the likes:

    Phase II: Step 3: Delete Symlink Operation Implementted

And  case you have multiple commits for the same step, reference it as a review as in.

    Phase II: Step 3:  Review A: Delete Symlink Operation Implementted

Like always, we must enssure the we don't break the test suite and that code adhere to our standards :

If, in your envrioment the git pre-commit hook is not installed, you can do so with:

    scripts/pre-commit install

Which are equivalent to :

    scripts/lint
    scripts/test

Do read the docs/intro-to-synthfs.txxxt file for a prmier on the concepts and limitations of the system.

## Current Status

**Implemented:**

- ✅ CreateFile operation (`pkg/synthfs/ops/create_file.go`)
- ✅ CreateDirectory operation (`pkg/synthfs/ops/create_dir.go`)
- ✅ SerializableCreateFile operation for JSON serialization

**Architecture Pattern:**

```go
// Existing pattern that works well
op := ops.NewCreateFile("config.json", []byte("data"), 0644)
    .WithID("create-config")
    .WithDependency("create-dir")
```

## Implementation Strategy

We'll implement foundation work first to establish consistent patterns, then expand operation by operation to ensure all targets follow the same design.

### Phase 0: Foundation Work

**Goal:** Establish the hybrid API pattern without breaking existing code.

**Deliverables:**

1. **Define FsItem interface and builders** (`pkg/synthfs/items.go`)

   ```go
   type FsItem interface {
       Path() string
       Type() string
   }
   
   type FileItem struct { path, content, mode }
   type DirectoryItem struct { path, mode }
   type SymlinkItem struct { path, target }
   type ArchiveItem struct { path, format, sources }
   
   // Builders with fluent API
   func NewFile(path string) *FileItem
   func NewDirectory(path string) *DirectoryItem
   func NewSymlink(path string) *SymlinkItem  
   func NewArchive(path string) *ArchiveItem
   ```

2. **Generic operation wrappers** (`pkg/synthfs/ops/generic.go`)

   ```go
   func Create(item FsItem) Operation
   func Delete(path string) Operation
   func Copy(src, dst string) Operation  
   func Move(src, dst string) Operation
   ```

3. **Verification with existing operations**
   - Ensure `Create(NewFile(...))` produces same result as `NewCreateFile(...)`
   - All existing tests continue to pass
   - Both APIs work interchangeably

### Phase 1: Complete Create Operations

**Goal:** Support creation of all filesystem item types.

**Deliverables:**

1. **CreateSymlink operation** (`pkg/synthfs/ops/create_symlink.go`)
   - Constructor: `NewCreateSymlink(path, target string)`
   - Chainable: `.WithID()`, `.WithDependency()`
   - Validation: Check target exists, path is valid
   - Execution: Use `FileSystem.Symlink()`
   - Rollback: Remove created symlink

2. **CreateArchive operation** (`pkg/synthfs/ops/create_archive.go`)
   - Constructor: `NewCreateArchive(path string, format ArchiveFormat, sources []string)`
   - Support formats: tar.gz, zip initially  
   - Validation: Check all sources exist
   - Execution: Create archive from sources
   - Rollback: Remove created archive

3. **Archive format support** (`pkg/synthfs/archive.go`)

   ```go
   type ArchiveFormat int
   const (
       ArchiveFormatTarGz ArchiveFormat = iota
       ArchiveFormatZip
   )
   ```

4. **Integration with generic Create()**
   - Update `Create()` to handle SymlinkItem and ArchiveItem
   - Comprehensive test coverage

### Phase 2: Delete Operations

**Goal:** Support deletion of all filesystem item types.

**Deliverables:**

1. **DeleteFile operation** (`pkg/synthfs/ops/delete_file.go`)
2. **DeleteDirectory operation** (`pkg/synthfs/ops/delete_dir.go`)  
3. **DeleteSymlink operation** (`pkg/synthfs/ops/delete_symlink.go`)
4. **DeleteArchive operation** (`pkg/synthfs/ops/delete_archive.go`)

**Shared patterns:**

- Constructor: `NewDeleteFile(path string)`, etc.
- Validation: Check item exists and is correct type
- Execution: Use appropriate `FileSystem.Remove*()` method
- Rollback: Restore deleted item (challenging - may need backup)

### Phase 3: Copy Operations

**Goal:** Support copying between filesystem items.

**Deliverables:**

1. **CopyFile operation** (`pkg/synthfs/ops/copy_file.go`)
2. **CopyDirectory operation** (`pkg/synthfs/ops/copy_dir.go`)
3. **CopySymlink operation** (`pkg/synthfs/ops/copy_symlink.go`)
4. **CopyArchive operation** (`pkg/synthfs/ops/copy_archive.go`)

**Shared patterns:**

- Constructor: `NewCopyFile(src, dst string)`, etc.
- Validation: Source exists, destination path valid
- Execution: Type-appropriate copy logic
- Rollback: Remove copied item

### Phase 4: Move Operations

**Goal:** Support moving/renaming filesystem items.

**Deliverables:**

1. **MoveFile operation** (`pkg/synthfs/ops/move_file.go`)
2. **MoveDirectory operation** (`pkg/synthfs/ops/move_dir.go`)
3. **MoveSymlink operation** (`pkg/synthfs/ops/move_symlink.go`)
4. **MoveArchive operation** (`pkg/synthfs/ops/move_archive.go`)

**Shared patterns:**

- Constructor: `NewMoveFile(src, dst string)`, etc.
- Validation: Source exists, destination path valid
- Execution: Use `FileSystem.Rename()` where possible
- Rollback: Move back to original location

## Implementation Notes

### FileSystem Interface Extensions

May need to extend `FileSystem` interface for new operations:

```go
type FileSystem interface {
    ReadFS
    WriteFS
    
    // Existing
    WriteFile(name string, data []byte, perm fs.FileMode) error
    MkdirAll(path string, perm fs.FileMode) error
    Remove(name string) error
    RemoveAll(name string) error
    
    // New for symlinks
    Symlink(oldname, newname string) error
    Readlink(name string) (string, error)
    
    // New for moves
    Rename(oldpath, newpath string) error
}
```

### Testing Strategy

Each operation needs:

- Unit tests for constructor and methods
- Integration tests with MockFS
- Error case testing
- Rollback functionality testing
- Cross-operation dependency testing

### Documentation

Each phase should include:

- API documentation for new operations
- Usage examples
- Migration guide (if any breaking changes)
- Updated README with supported operations matrix

## Success Criteria

- All operations work with both old and new APIs
- Comprehensive test coverage (>90%)
- Consistent behavior patterns across operations
- Full rollback support for transactional execution
- Performance benchmarks for archive operations
- CLI integration for all operation types
