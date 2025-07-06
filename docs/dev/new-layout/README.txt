Go SynthFS Restructuring Plan
===============================

the current codebase grew organically , and , in the process, the codelayout is pretty bad, with 1 file being 1.3kloc and 30% . of the code base. 

we're now decided on a new format, and this branch is about getting there. 



Target Structure:
pkg/synthfs/
├── types.go              # Core types and interfaces
├── constants.go          # All enums and constants  
├── errors.go             # All error types
├── 
├── targets/              # Target types (what we operate on)
│   ├── file.go           # FileItem
│   ├── directory.go      # DirectoryItem
│   ├── symlink.go        # SymlinkItem
│   └── archive.go        # ArchiveItem, UnarchiveItem
├── 
├── operations/           # Operations (what we do to targets)
│   ├── create.go         # Create operations
│   ├── copy.go           # Copy operations
│   ├── move.go           # Move operations
│   ├── delete.go         # Delete operations
│   └── archive.go        # Archive/unarchive operations
├── 
├── execution/            # How operations get executed
│   ├── pipeline.go       # Pipeline interface and implementation
│   ├── executor.go       # Executor
│   ├── batch.go          # Batch API
│   └── state.go          # State tracking
├── 
├── backup/               # Backup/restore functionality
│   ├── backup.go         # Backup system
│   └── restore.go        # Restoration logic
├── 
├── filesystem/           # Filesystem abstractions
│   ├── interfaces.go     # FileSystem interfaces
│   ├── os.go            # OS filesystem
│   └── memory.go        # Memory/test filesystem
├── 
├── validation/           # Validation and verification
│   ├── checksum.go      # Checksum functionality
│   └── validator.go     # Validation logic
├── 
├── log.go               # Logging (small enough to stay at root)
└── testing.go           # Testing utilities (small enough to stay at root)

Key Design Decisions:
- types.go for core types/interfaces (idiomatic Go)
- constants.go for all enums/constants (consistent string-based enums)
- errors.go for all error types (centralized error handling)
- Domain-driven organization: targets/operations/execution
- Clear separation: what (targets) vs how (operations) vs when (execution)
- Backup gets own package (substantial feature)
- Filesystem abstractions isolated
- String-based enums over iota (more debuggable)

Migration Steps:
1. Mapping DONE
	1.1 List all types / function as they exists now and where they will be after the refactor
	1.2 List all tests , categorize each one (and, looking at the final changed layout, propose) which file it should be (you'll have to create many of these)
	1.3 Create a file docs/dev/new-layout/code-mapping.md and put 1.1. there (the mappng of each old thing to the new path)
	1.4 Create a file docs/dev/new-layout/tests and list groupped by files, where each test should go. 

	Note that we should have the same number of tests, we don't want to refactor, just redistrubute them. Ditto for code, no new code.

2.  Shared things DONE
	2.1 Extract types to types.go
	2.2 Extract constants to constants.go  
	2.3 Extract errors to errors.go

	Make sure that all test pass

3. For each new layout direcotry (targets, operations, execution, etc)
	3.1 Create new directory structure
	3.2 Move implementation code to appropriate packages
	3.3. Update imports throughout codebase
	3.4 Remove old files.
	3.5. Verify tests pass
	3.6. Update documentation
	Commit and push to git.

	3.a targets/ - DONE
		- Created targets/ directory
		- Moved FileItem, DirectoryItem, SymlinkItem, ArchiveItem to respective files
		- Created interface.go for FsItem interface
		- Moved tests to targets/*_test.go
		- All tests passing

	3.b operations/ - ATTEMPTED BUT REVERTED, THEN RESOLVED DIFFERENTLY
		- Created operations/ directory structure
		- Encountered circular dependency between synthfs and operations packages
		- Reverted changes to maintain working state
		- RESOLVED: Split operation.go into multiple files within synthfs package:
		  * operation_simple.go - SimpleOperation struct and core methods
		  * operation_create.go - create file/dir/symlink operations
		  * operation_delete.go - delete operations
		  * operation_copy_move.go - copy and move operations
		  * operation_archive.go - archive/unarchive operations
		  * operation_reverse.go - reverse operations for Phase 3
		  * operation_backup.go - backup budget methods
		- Fixed all test failures after refactoring
		- All 235 tests passing

	3.c execution/ - ATTEMPTED BUT FAILED
		- Cannot create execution package due to circular dependencies
		- Execution types (Executor, Pipeline, PathStateTracker) need many types from synthfs
		- But synthfs/batch.go needs to use these execution types
		- Creating import cycle: synthfs → execution → synthfs
		- RESOLUTION: Keep executor.go, pipeline.go, state.go in main synthfs package

	3.d backup/ - NOT STARTED
		- Will extract backup/restore functionality from operation.go

	3.e filesystem/ - DONE
		- Created filesystem/ directory
		- Moved FileSystem interfaces to filesystem/interfaces.go
		- Moved OSFileSystem to filesystem/os.go
		- Moved ReadOnlyWrapper to filesystem/wrapper.go
		- Updated types.go to use type aliases for backward compatibility
		- Left TestFileSystem in testing.go (appropriate for test utilities)
		- Left ComputeFileChecksum in fs.go (small utility function)
		- All tests passing

	3.f validation/ - DONE
		- Created validation/ directory
		- Moved ChecksumRecord type to validation/checksum.go
		- Moved ComputeFileChecksum to validation/checksum.go
		- Updated types.go with type alias for backward compatibility
		- fs.go now just wraps the validation package function
		- All tests passing

Files to Split:
- operation.go (1600+ lines) -> operations/ + backup/ + types.go + errors.go
- items.go -> targets/ + types.go + constants.go
- fs.go -> filesystem/ + types.go
- state.go -> execution/ + types.go + constants.go
- executor.go -> execution/ + types.go + constants.go

Current Issues Addressed:
- Scattered type definitions
- Inconsistent enum patterns
- Mixed concerns in large files
- Poor discoverability of core types
- No centralized constants

NEXT RECOMMENDED STEP:
====================
The operations/ package failed due to circular dependencies. Before proceeding with more package splits,
we should consider one of these approaches:

1. Keep operations in the main synthfs package (current state)
   - SimpleOperation stays in operation.go
   - Avoids circular dependency issues
   - Less ideal structure but functional

2. Create an internal/core package
   - Move shared interfaces/types to internal/core
   - Both synthfs and operations can import internal/core
   - Prevents external packages from importing internal/core

3. Restructure to avoid the dependency
   - Move operation creation logic out of batch.go
   - Use factory functions or builders
   - More complex refactoring

Recommendation: Skip operations/ for now and proceed with execution/ package,
which should have fewer dependency issues. The execution package can be created
without circular dependencies since it doesn't need to be imported by batch.go.

CURRENT STATUS:
==============
- operation.go successfully split from 1,510 lines to 7 focused files
- filesystem package created with clean separation of interfaces and implementations  
- validation package created for checksum functionality
- fs.go reduced from 186 lines to 29 lines (now just wrapper functions)
- execution package successfully created with executor, pipeline, and state logic
- Type aliases maintained for backward compatibility
- All tests passing (235 tests)

NEXT RECOMMENDED STEPS:
======================
Now that operation.go is refactored, we have two main large files left:
1. batch.go (1,041 lines) - Contains batch operations and dependency resolution
2. fs.go (368 lines) - Contains filesystem interfaces and implementations

Option 1: Split batch.go next
- Move pipeline interface and implementation to execution/pipeline.go
- Move executor to execution/executor.go  
- Move state tracking to execution/state.go
- Keep batch.go but reduce its size
- Challenge: batch.go has tight coupling with operations

Option 2: Split fs.go next (COMPLETED ✓)
- Moved FileSystem interfaces to filesystem/interfaces.go
- Moved OSFileSystem to filesystem/os.go
- Moved ReadOnlyWrapper to filesystem/wrapper.go
- fs.go now only contains utility functions
- Clean separation achieved with no circular dependencies

Option 3: Extract validation logic
- Create validation/ package
- Move checksum functionality from various files
- Move validation logic from operations
- This is relatively independent and low risk

UPDATED RECOMMENDATIONS (after filesystem refactoring):
=====================================================
With filesystem package complete, we should now tackle:

Option A: Create validation/ package (COMPLETED ✓)
- Moved ComputeFileChecksum from fs.go
- Created checksum.go with ChecksumRecord type and ComputeFileChecksum
- fs.go now contains only wrapper functions
- Clean separation achieved

Option B: Start on execution/ package
- Move pipeline.go content (currently in batch.go)
- Move executor.go to execution/executor.go
- Move state.go to execution/state.go
- More complex due to interdependencies

Option C: Extract backup/ package
- Move backup-related code from operation_backup.go and operation_reverse.go
- Create backup/budget.go, backup/restore.go
- Medium complexity

NEW RECOMMENDATIONS (after validation package complete):
=======================================================
With validation, filesystem, and operation refactoring done:

Option 1: Tackle execution/ package (COMPLETED ✓)
- Successfully moved executor, pipeline, and state logic to execution/
- Main package files now contain only type aliases and wrappers
- Clean separation achieved with backward compatibility

Option 2: Extract backup/ package
- Move BackupBudget from types.go
- Move operation_backup.go content
- Move backup-related parts of operation_reverse.go
- Create cohesive backup functionality package

Option 3: Clean up remaining small files
- fs.go (29 lines) could be merged into another file or removed
- Consider if any other small consolidations make sense

SUMMARY OF COMPLETED WORK:
=========================

1. targets/ package (DONE)
   - Successfully extracted all item types
   - Clean separation of target types

2. operations/ refactoring (DONE)
   - Split operation.go into 7 focused files within synthfs package
   - Avoided circular dependency issues by keeping in main package

3. filesystem/ package (DONE)
   - Extracted all filesystem interfaces and implementations
   - Clean separation with type aliases for compatibility

4. validation/ package (DONE)
   - Extracted checksum functionality
   - Minimal fs.go now just contains wrappers

5. execution/ package (FAILED)
   - Cannot extract due to circular dependencies
   - These components are too tightly integrated with core types

LESSONS LEARNED ABOUT CIRCULAR DEPENDENCIES:
==========================================

The attempt to create an execution package revealed Go's strict import rules:

1. **Why execution package failed:**
   - Executor, Pipeline, PathStateTracker need types like Operation, FileSystem, Logger from synthfs
   - batch.go in synthfs needs to use Executor, Pipeline types
   - This creates: synthfs → execution → synthfs (circular!)

2. **When package extraction works:**
   ✓ Self-contained types with minimal dependencies (targets/)
   ✓ Utility functions that don't import the main package (filesystem/, validation/)
   ✓ Splitting large files within same package (operation_*.go)

3. **When package extraction fails:**
   ✗ Core types used throughout the codebase
   ✗ Components that need many types from main package
   ✗ Tightly coupled business logic (executor, pipeline, state)

4. **Alternative approaches considered:**
   - Type aliases and wrappers (still creates circular dependency)
   - Interface segregation (too complex for current needs)
   - Keep as-is (chosen solution)

FINAL RECOMMENDATIONS:
=====================

1. **Accept current structure:**
   - executor.go, pipeline.go, state.go are well-organized within synthfs
   - They're already separate files with clear responsibilities
   - Moving them provides no real benefit given the complexity

2. **Consider backup/ package:**
   - Backup logic might be extractable if it has fewer dependencies
   - Would need careful analysis to avoid same circular dependency issues

3. **Focus on what worked:**
   - targets/ package successfully extracted
   - filesystem/ package successfully extracted
   - validation/ package successfully extracted
   - operation.go successfully split into 7 files

CONCLUSION:
==========
The refactoring achieved significant improvements:
- Reduced operation.go from 1,510 to 7 focused files
- Created clean packages for targets, filesystem, and validation
- Improved code organization and maintainability
- All 235 tests still passing

The execution components remain in synthfs due to Go's import constraints, which is an acceptable trade-off.