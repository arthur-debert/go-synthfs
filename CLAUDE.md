# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build, Test, and Lint
```bash
# Build all packages and CLI binary
./scripts/build

# Run all tests with coverage
./scripts/test

# Run tests in CI mode (with coverage threshold checking - currently disabled)
./scripts/test --ci

# Run linter
./scripts/lint

# Install pre-commit hooks
./scripts/pre-commit install
```

### Development Commands
```bash
# Count lines of Go code
./scripts/cloc-go [directory]  # defaults to pkg/

# Create a new release
./scripts/release-new [--major|--minor|--patch] [--yes]

# Run a single test
go test -v -run TestName ./pkg/synthfs/...
```

## High-Level Architecture

### Core Concept: Lazy Evaluation Filesystem Operations

SynthFS provides a **transactional-like approach** to filesystem operations through lazy evaluation:
1. **Operations** are created and validated upfront but not executed
2. Operations are collected into **Pipelines** with dependency tracking
3. Execution happens all at once with **best-effort rollback** on failure

This is NOT a true transactional filesystem - it's an optimistic, best-effort system designed for practical use cases where you want to validate a series of filesystem changes before executing them.

### Package Structure

```
pkg/synthfs/
├── operations/     # Individual operation implementations (CreateFile, Copy, Move, Delete, etc.)
├── targets/        # Filesystem item types (File, Directory, Symlink, Archive)
├── testutil/       # Testing utilities and mock filesystems
├── fs.go          # FileSystem interfaces (ReadFS, WriteFS)
├── operation.go   # Core Operation interface and SimpleOperation base
├── executor.go    # Executes operations with rollback support
├── pipeline.go    # Groups operations with dependency management
└── *.go           # Other core implementation files
```

### Key Design Principles

1. **Validation at Creation Time**: Operations validate their parameters when created, not during execution
2. **Immutable Operations**: Once created, operations cannot be modified
3. **Simple Mental Model**: Queue operations → Validate → Execute with rollback
4. **No Guarantees**: Best-effort execution and rollback, not ACID transactions

### Current Development Status

The project recently completed **Phase 0.5** which removed 486 lines of over-engineered code to align with the simple vision. Currently working on **Phase 1A** to extend the FileSystem interface with symlink and rename support, which will unblock implementation of remaining operations.

Key recent changes:
- Removed complex progress reporting system
- Simplified executor to basic Execute(ctx, queue, filesystem) method
- Replaced complex BaseOperation with SimpleOperation
- Maintained backward compatibility during transition

### Testing Strategy

- Comprehensive test suite using `testutil/mock_fs.go` for filesystem mocking
- Tests focus on operation validation, execution, and rollback scenarios
- Use `go test -v -run TestName ./pkg/synthfs/...` to run specific tests
- Coverage reports generated with `./scripts/test`

### Important Notes

- This is a library, not just a CLI tool - the CLI in `cmd/synthfs/` is minimal
- The main branch is `main`, not `master`
- Pre-commit hooks are available but optional (`./scripts/pre-commit install`)
- The project uses semantic versioning with git tags