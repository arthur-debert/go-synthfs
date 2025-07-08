# Operation-Driven Prerequisites Design

This document contais the full redesign of fsynth's execution.
A few pointers on the impelemntation:

1. Commit granularly, at least 1 commit per milestone (a phase has several milestones. ) . It's fine if a milestone generates various commits, but do not clump together various milestones into a single commit.
2. For each commit message use the tetmplate : Execution Refac <phase> <milestone> Description
3. as you make progress, do add (DONE) to the milestone description here for tracking.
4. scripts/test, scripts/lint are needed for commiting, both are ran on the scripts/pre-commit hook. I strong advise you to run these as you go as not to be caught up into reverting too big a change.
5. commiting with --no-verify is a no no. never.

## Problem Statement

The current batch/execution design has several issues:

1. **Batch knows too much**: Hardcoded operation types, parent directory creation logic
2. **Operations are passive**: They don't declare their prerequisites
3. **Tight coupling**: Batch implementation depends on specific operation types
4. **Overlapping responsibilities**: Validation, dependency management split across components

## Proposed Solution

Operations declare their prerequisites, execution pipeline resolves them generically.

## Core Design

### 1. Prerequisite Interface (in core package to avoid circular deps)

```go
// pkg/synthfs/core/prerequisites.go
package core

// Prerequisite represents a condition that must be met before an operation executes
type Prerequisite interface {
    Type() string        // "parent_dir", "no_conflict", "source_exists"
    Path() string        // Path this prerequisite relates to
    Validate(fsys interface{}) error
}

// PrerequisiteResolver can create operations to satisfy prerequisites
type PrerequisiteResolver interface {
    CanResolve(prereq Prerequisite) bool
    Resolve(prereq Prerequisite) ([]interface{}, error) // Returns operations
}
```

### 2. Enhanced Operation Interface

```go
// Add to operations.Operation interface
type Operation interface {
    // ... existing methods ...
    
    // Prerequisites returns what this operation needs
    Prerequisites() []core.Prerequisite
}
```

### 3. Concrete Prerequisites

```go
// pkg/synthfs/core/prerequisites_impl.go
type ParentDirPrerequisite struct {
    path string
}

func (p *ParentDirPrerequisite) Type() string { return "parent_dir" }
func (p *ParentDirPrerequisite) Path() string { return p.path }
func (p *ParentDirPrerequisite) Validate(fsys interface{}) error {
    // Check if parent exists
}

type NoConflictPrerequisite struct {
    path string
}
// ... implementation
```

## Implementation Plan

### Phase 1: Add Prerequisites to Core (No Breaking Changes) (DONE)

**Goal**: Introduce prerequisite types without changing existing behavior

1. Add `core/prerequisites.go` with interfaces (DONE)
2. Add `core/prerequisites_impl.go` with concrete types (DONE)
3. Add default `Prerequisites() []core.Prerequisite { return nil }` to operations.BaseOperation (DONE)
4. **Tests**: All existing tests pass, no behavior change (DONE)

### Phase 2: Operations Declare Prerequisites (No Breaking Changes) (DONE)

**Goal**: Operations declare needs, but batch still handles them

1. Update CreateFileOperation to return ParentDirPrerequisite (DONE)
2. Update other operations to declare prerequisites (DONE)
3. Add unit tests for prerequisite declarations (DONE)
4. **Tests**: New tests for prerequisites, existing tests still pass (DONE)

### Phase 3: Add Prerequisite Resolution to Pipeline (No Breaking Changes) (DONE)

**Goal**: Pipeline can resolve prerequisites, but feature is opt-in

1. Create `execution/prerequisite_resolver.go` (DONE)
2. Add resolver that can create parent directory operations (DONE)
3. Add `ResolvePrerequisites bool` option to PipelineOptions (DONE)
4. When false (default), use existing batch behavior (DONE)
5. **Tests**: Add tests for new resolver, existing tests unchanged (DONE)

### Phase 4: Create SimpleBatch Alternative (No Breaking Changes) (DONE)

**Goal**: New simplified batch that doesn't handle prerequisites

1. Create `batch/simple_batch.go` as new implementation (DONE)
2. No parent dir logic, just creates operations (DONE)
3. Add `NewSimpleBatch()` constructor (DONE)
4. Existing `NewBatch()` returns current implementation (DONE)
5. **Tests**: New tests for SimpleBatch, old batch tests unchanged (DONE)

### Phase 5: Migration Path (No Breaking Changes) (DONE)

**Goal**: Allow gradual migration to new design

1. Add `UseSimpleBatch bool` to batch options (DONE)
2. When true, use SimpleBatch + prerequisite resolution (DONE)
3. When false (default), use existing behavior (DONE)
4. Update documentation with migration guide (DONE)
5. **Tests**: Integration tests for both paths (DONE)

### Phase 6: Switch Defaults (Controlled Breaking Change) (DONE)

**Goal**: Make new behavior default, deprecate old

1. Change `UseSimpleBatch` default to true (DONE)
2. Add deprecation notices to old batch methods (DONE)
3. Update all internal usage to new pattern (DONE)
4. **Tests**: Update tests to use new pattern primarily (DONE)

### Phase 7: Cleanup (Major Version) (FUTURE)

**Goal**: Remove old implementation

1. Remove old batch implementation
2. Remove compatibility flags
3. Simplify codebase
4. **Tests**: Remove old test paths

## Circular Import Prevention Strategy

### Package Hierarchy (Strict)

```
core/           (no imports from synthfs)
    ↑
operations/     (imports core only)
    ↑
execution/      (imports core only)
    ↑
batch/          (imports core, operations)
    ↑
synthfs/        (imports all)
```

### Rules

1. **core** package has NO imports from synthfs packages
2. **operations** imports core only, uses interface{} for filesystem
3. **execution** imports core only, uses interface{} for operations
4. **batch** can import operations and core, but not synthfs types
5. **synthfs** root package does all type conversions/adapters

### Interface Segregation

- Duplicate small interfaces rather than import
- Use interface{} with type assertions at boundaries
- Keep prerequisite types in core to avoid circular deps

## Benefits

1. **Extensibility**: New operation types just implement Prerequisites()
2. **Testability**: Each component has single responsibility
3. **Maintainability**: No hardcoded operation knowledge in batch
4. **Flexibility**: Operations can declare complex prerequisites

## Risks & Mitigations

1. **Risk**: Circular imports during implementation
   - **Mitigation**: Strict package hierarchy, interface{} at boundaries

2. **Risk**: Performance overhead from prerequisite checking
   - **Mitigation**: Cache prerequisite validation results

3. **Risk**: Complex migration for users
   - **Mitigation**: Gradual phases, compatibility flags

## Success Criteria

1. Batch no longer has hardcoded operation type strings
2. Operations explicitly declare all prerequisites  
3. New operation types can be added without modifying batch/pipeline
4. All existing tests pass throughout migration
5. No circular import issues introduced

## Status Update

**COMPLETED**: All phases through Phase 6 are now complete! 

The entire prerequisite-driven execution redesign has been successfully implemented:

✅ **Phase 1**: Prerequisite interfaces and implementations in core package
✅ **Phase 2**: All operations declare their prerequisites
✅ **Phase 3**: Pipeline prerequisite resolution with opt-in flag
✅ **Phase 4**: SimpleBatch implementation without parent dir auto-creation
✅ **Phase 5**: Migration path with UseSimpleBatch option
✅ **Phase 6**: SimpleBatch is now the default behavior

**Key Features Now Available:**
- Operations explicitly declare prerequisites (ParentDir, NoConflict, SourceExists)
- Pipeline automatically resolves prerequisites by creating necessary operations
- SimpleBatch provides clean separation of concerns
- Migration path supports both old and new behavior
- All success criteria have been met

**Phase 7** (cleanup) can be done in the future when ready to remove legacy code.

The system now supports:
- Extensible prerequisite system
- Clean separation between batch and prerequisite logic
- Automatic parent directory creation via prerequisites
- Full backward compatibility during migration
