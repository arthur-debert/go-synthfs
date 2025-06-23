# Operations Implementation Plan

This document outlines the implementation plan for expanding synthfs to support creation, deletion, copying, and moving of files, directories, symlinks, and archives.

This is a signifcant change in design. This is yet unreleased software, which means there is no backwards compatibility to keep, in fact, it's part of the work to adapt the current code and tests to the new design.  This is much cheaper than forking and keeping two concurrent designs for which one has no users. Don't keep backwards compatibility.

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

Do read the `docs/dev/intro-to-synthfs.txt` file for a primer on the concepts and limitations of the system.

## Current Status

**Implemented:**

- ✅ CreateFile operation (`pkg/synthfs/ops/create_file.go`)
- ✅ CreateDirectory operation (`pkg/synthfs/ops/create_dir.go`)
- ✅ SerializableCreateFile operation for JSON serialization

**Architecture Pattern:**

```golang
// Existing pattern that works well
op := ops.NewCreateFile("config.json", []byte("data"), 0644)
    .WithID("create-config")
    .WithDependency("create-dir")
```

## Implementation Strategy

We'll implement foundation work first to establish consistent patterns, then expand operation by operation to ensure all targets follow the same design.

### API Design Philosophy

To provide a powerful and consistent API, we will adopt a single, unified strategy. The legacy builder pattern (`ops.NewCreateFile`) will be removed.

1. **Item-Based Creation:** For `Create` operations, the caller will use `FsItem` objects (`NewFile`, `NewDirectory`) to provide a declarative, type-safe way to define what should be created.

2. **Unified Manipulation:** For `Delete`, `Copy`, and `Move` operations, the caller will use simple, unified functions that operate on paths. These operations will be smart enough to inspect the path and perform the correct action for the given filesystem item (file, directory, etc.).

### Phase 0: Foundation Work

**Goal:** Establish the new unified API and refactor existing code to use it.

**Deliverables:**

1. **Define FsItem interface and builders** (`pkg/synthfs/items.go`)

   ```golang
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

2. **Unified operation constructors** (`pkg/synthfs/ops/generic.go`)

   ```golang
   func Create(item FsItem) Operation
   func Delete(path string) Operation
   func Copy(src, dst string) Operation  
   func Move(src, dst string) Operation
   ```

3. **Refactor Existing Code**
   - Refactor `CreateFile` and `CreateDirectory` operations to use the new `Create(FsItem)` pattern.
   - Update all existing tests to use the new API.
   - Remove the old builder-style constructors (`ops.NewCreateFile`, etc.).

### Phase 1: Complete Create Operations

**Goal:** Support creation of all filesystem item types using the new API.

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

   ```golang
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

**Goal:** Support deletion of any filesystem item using a single, unified operation.

**Deliverables:**

1. **Unified Delete operation** (`pkg/synthfs/ops/delete.go`)
   - Constructor: `NewDelete(path string)`
   - Validation: Check that the path exists.
   - Execution: Inspect the item at `path` and use the appropriate `FileSystem` method (`Remove` for files/symlinks, `RemoveAll` for directories).
   - Rollback: Restore the deleted item. This is challenging and may require caching the item's content (for files) or metadata before deletion. A robust strategy for directory rollback needs to be designed.

### Phase 3: Copy Operations

**Goal:** Support copying any filesystem item using a single, unified operation.

**Deliverables:**

1. **Unified Copy operation** (`pkg/synthfs/ops/copy.go`)
   - Constructor: `NewCopy(src, dst string)`
   - Validation: Check that the source path exists and the destination is valid.
   - Execution: Inspect the item at `src` and perform a type-appropriate copy (e.g., recursive for directories).
   - Rollback: Remove the item created at `dst`.

### Phase 4: Move Operations

**Goal:** Support moving/renaming any filesystem item using a single, unified operation.

**Deliverables:**

1. **Unified Move operation** (`pkg/synthfs/ops/move.go`)
    - Constructor: `NewMove(src, dst string)`
    - Validation: Check that the source path exists and the destination is valid.
    - Execution: Use `FileSystem.Rename()` for atomic moves. If `Rename` is not possible across devices, this may need to be implemented as a Copy-then-Delete operation.
    - Rollback: Move the item from `dst` back to `src`.

## Implementation Notes

### FileSystem Interface Extensions

May need to extend `FileSystem` interface for new operations:

```golang
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

- A single, consistent, and declarative API for all filesystem operations.
- Comprehensive test coverage (>90%)
- Consistent behavior patterns across operations
- Full rollback support for transactional execution
- Performance benchmarks for archive operations
- CLI integration for all operation types
