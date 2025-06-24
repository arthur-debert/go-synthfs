# SynthFS Development Plan

## Overview

This document outlines the development strategy for completing the SynthFS library. After consolidating the v2 and legacy codebases, we're taking an **operations-first approach** to maximize value delivery and minimize rework.

## 🎯 Strategic Approach: Operations-Led Development

### Why Operations First?

1. **Lower Risk, Higher Certainty** - Operations have clear, testable specifications
2. **Concrete Requirements Drive Better Design** - Infrastructure designed after seeing real usage patterns
3. **Faster Value Delivery** - Each operation provides immediate user value
4. **Development Momentum** - Clear "done" criteria and independent deliverables

### Critical Exception: Foundation Infrastructure

Some infrastructure is a **prerequisite** for operations and must be done first.

---

## 📋 Development Phases

### **Phase 1A: Critical FileSystem Extensions** ⏱️ 1 week

**Goal:** Unblock Phase 1 operations by extending the FileSystem interface.

**Deliverables:**

1. **Extend WriteFS Interface** (`pkg/synthfs/fs.go`)

   ```golang
   // Add to WriteFS interface:
   Symlink(oldname, newname string) error    // For CreateSymlink operation
   Readlink(name string) (string, error)     // For symlink validation/rollback  
   Rename(oldpath, newpath string) error     // For Move operations
   ```

2. **Update OSFileSystem Implementation**

   ```golang
   // pkg/synthfs/fs.go - Add implementations:
   func (osfs *OSFileSystem) Symlink(oldname, newname string) error
   func (osfs *OSFileSystem) Readlink(name string) (string, error) 
   func (osfs *OSFileSystem) Rename(oldpath, newpath string) error
   ```

3. **Update TestFileSystem** (`pkg/synthfs/testing.go`)
   - Add mock implementations for testing
   - Ensure symlink/rename behavior is testable

4. **Update MockFS** (`pkg/synthfs/testutil/mock_fs.go`)
   - Add symlink support to mock filesystem
   - Add rename operation support

**Success Criteria:**

- [ ] All FileSystem interfaces compile
- [ ] OSFileSystem supports symlinks and renames
- [ ] TestFileSystem supports symlinks and renames
- [ ] MockFS supports symlinks and renames
- [ ] Full test coverage for new methods

---

### **Phase 1B: Complete Create Operations** ⏱️ 2-3 weeks

**Goal:** Implement CreateSymlink and CreateArchive operations to complete Phase 1.

**Deliverables:**

1. **CreateSymlink Operation** (`pkg/synthfs/ops/create_symlink.go`)

   ```golang
   // Will use the generic Create() with SymlinkItem
   // But may need specific implementation details for validation
   ```

   - Validation: Check target exists, path is valid
   - Execution: Use `FileSystem.Symlink()`
   - Rollback: Remove created symlink
   - Integration with `ops.Create(synthfs.NewSymlink(...))`

2. **CreateArchive Operation** (`pkg/synthfs/ops/create_archive.go`)

   ```golang
   // Support tar.gz and zip formats initially
   ```

   - Validation: Check all sources exist
   - Execution: Create archive from sources (tar.gz, zip)
   - Rollback: Remove created archive
   - Integration with `ops.Create(synthfs.NewArchive(...))`

3. **Archive Implementation**
   - Archive creation logic (tar.gz, zip)
   - Proper error handling for missing sources
   - Support for both files and directories as sources

4. **Complete Generic Create() Integration**
   - Ensure `ops.Create()` properly handles SymlinkItem and ArchiveItem
   - Update operation description generation
   - Test all item types through generic interface

**Success Criteria:**

- [ ] `ops.Create(synthfs.NewSymlink(path, target))` works end-to-end
- [ ] `ops.Create(synthfs.NewArchive(path, format, sources))` works end-to-end
- [ ] Full validation, execution, and rollback for both operations
- [ ] Comprehensive test coverage
- [ ] Integration tests with real filesystem

---

### **Phase 2: Delete Operations** ⏱️ 2 weeks

**Goal:** Support deletion of any filesystem item using unified operation.

**Deliverables:**

1. **Unified Delete Operation** (`pkg/synthfs/ops/delete.go`)

   ```golang
   func Delete(path string) Operation
   // Uses ops.Delete() from generic.go, but may need specific implementation
   ```

   - Validation: Check that path exists
   - Execution: Inspect item type and use appropriate removal method
   - Rollback: **Advanced** - Store item content/metadata before deletion

2. **Intelligent Path Inspection**
   - Detect file vs directory vs symlink at path
   - Choose appropriate removal strategy (Remove vs RemoveAll)
   - Handle edge cases (broken symlinks, etc.)

3. **Rollback Strategy Design**
   - **Files**: Store content + metadata before deletion
   - **Directories**: Store full tree structure + contents  
   - **Symlinks**: Store target path
   - Implement restoration logic

**Success Criteria:**

- [ ] `ops.Delete("/path")` works for files, directories, symlinks
- [ ] Proper validation and error handling
- [ ] Basic rollback support (files and symlinks)
- [ ] Advanced rollback for directories (stretch goal)

---

### **Phase 3: Copy Operations** ⏱️ 2 weeks

**Goal:** Support copying any filesystem item using unified operation.

**Deliverables:**

1. **Unified Copy Operation** (`pkg/synthfs/ops/copy.go`)

   ```golang
   func Copy(src, dst string) Operation
   ```

   - Validation: Source exists, destination is valid
   - Execution: Type-appropriate copy (recursive for directories)
   - Rollback: Remove item created at destination

2. **Intelligent Copy Logic**
   - **Files**: Direct content copy with mode preservation
   - **Directories**: Recursive copy with structure preservation
   - **Symlinks**: Copy as symlink or copy target (configurable)

3. **Performance Considerations**
   - Efficient large file copying
   - Progress reporting for large operations
   - Memory usage optimization

**Success Criteria:**

- [ ] `ops.Copy("/src", "/dst")` works for all item types
- [ ] Preserves permissions, timestamps where possible
- [ ] Handles symlinks correctly
- [ ] Efficient for large files/directories

---

### **Phase 4: Move Operations** ⏱️ 1-2 weeks

**Goal:** Support moving/renaming filesystem items using unified operation.

**Deliverables:**

1. **Unified Move Operation** (`pkg/synthfs/ops/move.go`)

   ```golang
   func Move(src, dst string) Operation  
   ```

   - Validation: Source exists, destination is valid
   - Execution: Use `FileSystem.Rename()` when possible
   - Fallback: Copy-then-Delete for cross-device moves
   - Rollback: Move item back from destination to source

2. **Atomic vs Non-Atomic Moves**
   - Try atomic rename first
   - Fallback to copy+delete with proper error handling
   - Ensure consistency during multi-step operations

**Success Criteria:**

- [ ] `ops.Move("/src", "/dst")` works for all item types
- [ ] Atomic moves when possible
- [ ] Safe cross-device moves
- [ ] Proper rollback support

---

### **Phase 5: Minimal Serialization Infrastructure** ⏱️ 1-2 weeks

**Goal:** Enable basic plan persistence and CLI foundations.

**Deliverables:**

1. **Basic Serialization Interfaces** (`pkg/synthfs/serialization.go`)

   ```golang
   type SerializableOperation interface {
       Operation
       MarshalJSON() ([]byte, error)
       UnmarshalJSON([]byte) error
   }
   
   type OperationPlan struct {
       Operations []SerializableOperation `json:"operations"`
       Metadata   PlanMetadata            `json:"metadata"`
   }
   ```

2. **JSON Serialization for Core Operations**
   - Implement SerializableOperation for Create, Delete, Copy, Move
   - Plan marshaling/unmarshaling functions
   - Version handling for forward compatibility

3. **Basic Plan Execution**

   ```golang
   func ExecutePlan(ctx context.Context, plan *OperationPlan, fsys FileSystem) error
   ```

**Success Criteria:**

- [ ] Can serialize operation plans to JSON
- [ ] Can deserialize and execute plans
- [ ] Basic plan validation
- [ ] Simple plan file format established

---

### **Phase 6: CLI Plan Commands** ⏱️ 1-2 weeks

**Goal:** Restore CLI functionality with new operation system.

**Deliverables:**

1. **Plan Commands** (`cmd/synthfs/plan.go`)

   ```bash
   synthfs plan execute plan.json
   synthfs plan validate plan.json  
   synthfs plan create --output plan.json [operations...]
   ```

2. **Plan File Management**
   - Plan file validation
   - Error reporting for malformed plans
   - Dry-run execution mode

3. **Basic Plan Generation**
   - CLI flags to generate common operation patterns
   - Template system for common workflows

**Success Criteria:**

- [ ] Can execute plans from CLI
- [ ] Can validate plans without executing
- [ ] Good error messages and help text
- [ ] Integration with existing operation system

---

### **Phase 7+: Advanced Infrastructure** ⏱️ Future

**Deferred until after core operations are complete:**

1. **Advanced Rollback & Recovery**
   - Complete directory rollback with full state restoration
   - Transaction-like begin/commit/rollback interfaces
   - Recovery from partial failures

2. **Performance & Concurrency**
   - Parallel execution with conflict detection
   - Performance benchmarks for archive operations
   - Resource usage monitoring

3. **Advanced Validation & Analysis**
   - Automatic conflict detection between operations
   - Dependency graph visualization
   - Custom validation rules engine

4. **Documentation & Tooling**
   - Auto-generated documentation from plans
   - Migration tools for plan format changes
   - IDE integration and language server

---

## 🎯 Immediate Next Steps

### Current Sprint (Week 1)

**Priority 1: FileSystem Interface Extensions**

1. **Update WriteFS interface** in `pkg/synthfs/fs.go`
2. **Implement methods in OSFileSystem**
3. **Add symlink support to TestFileSystem**
4. **Add symlink support to MockFS**
5. **Write comprehensive tests**

**Concrete Tasks:**

- [ ] Add `Symlink(oldname, newname string) error` to WriteFS
- [ ] Add `Readlink(name string) (string, error)` to WriteFS  
- [ ] Add `Rename(oldpath, newpath string) error` to WriteFS
- [ ] Implement in OSFileSystem using `os.Symlink`, `os.Readlink`, `os.Rename`
- [ ] Update TestFileSystem with symlink tracking
- [ ] Update MockFS with symlink support
- [ ] Add test coverage for all new methods
- [ ] Update existing operation tests to use extended interface

### Success Metrics

**Technical:**

- All tests pass
- FileSystem interface properly extended
- No breaking changes to existing code

**Strategic:**

- Phase 1B operations can begin immediately after completion
- Clear path to completing all CRUD operations
- Infrastructure decisions informed by real operation usage

---

## 📊 Risk Management

### Technical Risks

1. **Symlink Support Complexity**
   - **Risk**: Cross-platform symlink behavior differences
   - **Mitigation**: Comprehensive test suite, platform-specific handling

2. **Archive Operation Performance**
   - **Risk**: Large archive creation impacts performance
   - **Mitigation**: Streaming implementation, progress reporting

3. **Rollback Complexity**
   - **Risk**: Directory rollback is complex and error-prone
   - **Mitigation**: Start with simple cases, iterate based on real needs

### Strategic Risks

1. **Serialization Format Changes**
   - **Risk**: Plan format changes break existing plans
   - **Mitigation**: Version plan format, implement migration tools

2. **API Design Lock-in**
   - **Risk**: Early API decisions constrain future flexibility
   - **Mitigation**: Iterate API based on operation implementation learnings

---

## 📈 Success Criteria for Overall Plan

### Technical Goals

- [ ] All CRUD operations implemented (Create, Read/Validate, Update/Move, Delete)
- [ ] Unified, consistent API across all operations
- [ ] Full rollback support for transactional execution
- [ ] Comprehensive test coverage (>90%)
- [ ] Plan serialization and CLI integration

### Strategic Goals  

- [ ] Clear, maintainable codebase with no v2/legacy confusion
- [ ] Incremental value delivery at each phase
- [ ] Design decisions validated by real usage
- [ ] Strong foundation for advanced features

### User Experience Goals

- [ ] Simple, intuitive API for common operations
- [ ] Powerful CLI for complex workflows
- [ ] Excellent error messages and validation
- [ ] Comprehensive documentation and examples

This plan balances **immediate value delivery** with **long-term architectural soundness**, ensuring each phase builds naturally on the previous one while delivering concrete user benefits.
