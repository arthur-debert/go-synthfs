# Dependency Analysis: Why Operations and Execution Can't Be Extracted

## The Problem in Simple Terms

We're trying to move code into separate packages, but we've hit a fundamental issue: **circular dependencies**. In Go, if package A imports package B, then package B cannot import package A.

## The Core Issue: Everything Depends on Everything

```
┌─────────────────────────────────────────────────┐
│                  synthfs package                 │
│                                                  │
│  ┌─────────────┐      uses      ┌────────────┐ │
│  │   Batch     │ ───────────────►│ Operations │ │
│  └─────────────┘                 └────────────┘ │
│         │                               ▲        │
│         │ uses                    implements    │
│         ▼                               │        │
│  ┌─────────────┐                 ┌────────────┐ │
│  │  Executor   │                 │   Items    │ │
│  └─────────────┘                 └────────────┘ │
│         │                               ▲        │
│         │ uses                        uses      │
│         ▼                               │        │
│  ┌─────────────┐      needs      ┌────────────┐ │
│  │  Pipeline   │ ───────────────►│   Types    │ │
│  └─────────────┘                 └────────────┘ │
└─────────────────────────────────────────────────┘
```

## Why This is a Code Smell

You're absolutely right - this is a code smell. The issue is that we have:

### 1. **The God Interface Problem**
The `Operation` interface is trying to be everything:
```go
type Operation interface {
    // Identity
    ID() OperationID
    
    // Execution
    Execute(ctx, fs) error
    Validate(ctx, fs) error
    Rollback(ctx, fs) error
    
    // Data
    GetItem() FsItem
    GetChecksum() *ChecksumRecord
    
    // Relationships
    Dependencies() []OperationID
    Conflicts() []OperationID
    
    // Advanced Features
    ReverseOps(ctx, fs, budget) ([]Operation, *BackupData, error)
    
    // Metadata
    Describe() OperationDesc
}
```

This interface is doing too much! It's mixing:
- Business logic (Execute, Validate)
- Data access (GetItem, GetChecksum)
- Metadata (ID, Dependencies)
- Advanced features (ReverseOps)

### 2. **Central Hub Architecture**
Everything connects through the Operation interface:
- **Batch** creates operations
- **Executor** runs operations
- **Pipeline** manages operations
- **State** tracks what operations will do
- **Operations** implement all of the above

### 3. **Type Definitions in Wrong Place**
All core types are defined in the main package:
- `OperationID`, `OperationDesc`, `BackupData`
- Error types: `ValidationError`, `DependencyError`
- Constants: `ArchiveFormat`, `PathStateType`

These are used everywhere, creating import dependencies.

## The Circular Dependency Pattern

When we try to extract operations to a separate package:

```
synthfs package:
  - Defines Operation interface
  - Batch creates operations (needs to import operations package)
  
operations package:
  - Implements Operation interface (needs to import synthfs for the interface)
  - Uses types like OperationID, FileSystem (needs to import synthfs)
  
Result: synthfs → operations → synthfs (CIRCULAR!)
```

## Why This Happened

This is a common evolution pattern:
1. Start with a simple design
2. Add features incrementally
3. Everything gets added to the same place
4. Interfaces grow to accommodate new features
5. Eventually, everything depends on everything

## How to Fix It (High-Level)

### Option 1: Split the God Interface
```go
// In a 'core' package
type OperationMetadata interface {
    ID() OperationID
    Dependencies() []OperationID
    Describe() OperationDesc
}

// In an 'execution' package
type Executable interface {
    Execute(ctx, fs) error
    Validate(ctx, fs) error
}

// In operations package
type Operation interface {
    OperationMetadata
    Executable
    GetItem() FsItem
}
```

### Option 2: Dependency Inversion
Instead of concrete types, use interfaces everywhere:
```go
// Don't do this:
func NewBatch() *Batch {
    return &Batch{
        operations: []Operation{}, // Direct dependency
    }
}

// Do this:
func NewBatch(opFactory OperationFactory) *Batch {
    return &Batch{
        opFactory: opFactory, // Injected dependency
    }
}
```

### Option 3: Event-Driven Architecture
Instead of direct calls, use events:
```go
// Instead of executor directly calling operations
executor.Run(operations)

// Use events
eventBus.Publish(ExecuteOperationEvent{Op: op})
// Operation handler subscribes and executes
```

## The Real Issue

The fundamental problem is that the current design has **high cohesion** (everything works together) but also **high coupling** (everything depends on everything). Good design should have high cohesion but **low coupling**.

The `Operation` interface has become the "god object" of the system - it knows about execution, validation, data, relationships, and advanced features. This makes it impossible to separate concerns into different packages without circular dependencies.