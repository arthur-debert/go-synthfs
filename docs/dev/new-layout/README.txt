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

	3.b operations/ - ATTEMPTED BUT REVERTED
		- Created operations/ directory structure
		- Encountered circular dependency between synthfs and operations packages
		- Reverted changes to maintain working state
		- NEXT STEP: Need different approach to avoid circular dependencies

	3.c execution/ - NOT STARTED
		- Will contain: pipeline.go, executor.go, batch.go, state.go

	3.d backup/ - NOT STARTED
		- Will extract backup/restore functionality from operation.go

	3.e filesystem/ - NOT STARTED
		- Will contain: interfaces.go, os.go, memory.go from current fs.go and testing.go

	3.f validation/ - NOT STARTED
		- Will contain: checksum.go, validator.go

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