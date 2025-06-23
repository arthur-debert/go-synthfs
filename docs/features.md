# Currently Implemented Operations

## CreateFile Operation** (`create_file`)

**Location**: `pkg/synthfs/ops/create_file.go`

**Purpose**: Creates a file with specified content and permissions.

**Features**:

- **Constructor**: `ops.NewCreateFile(path string, data []byte, mode fs.FileMode)`
- **Chainable API**:
  - `.WithID(id)` - Set custom operation ID
  - `.WithDependency(depID)` - Add operation dependencies
- **Validation**: Checks path format, file mode permissions, content size limits
- **Execution**: Uses `FileSystem.WriteFile()` to create the file
- **Rollback**: Removes the created file via `FileSystem.Remove()`
- **Logging**: Comprehensive logging at all levels with content preview/hex dumps

**Example**:

```go
op := ops.NewCreateFile("config.json", []byte(`{"key": "value"}`), 0644)
    .WithID("create-config")
    .WithDependency("create-dir")
```

### 2. **CreateDir Operation** (`create_dir`)

**Location**: `pkg/synthfs/ops/create_dir.go`

**Purpose**: Creates directories (including parent directories) with specified permissions.

**Features**:

- **Constructor**: `ops.NewCreateDir(path string, mode fs.FileMode)`
- **Chainable API**: Same as CreateFile (`.WithID()`, `.WithDependency()`)
- **Behavior**: Uses `MkdirAll` semantics - creates parent directories automatically
- **Validation**: Checks path format, directory mode permissions, prevents `..` segments
- **Execution**: Uses `FileSystem.MkdirAll()`
- **Rollback**: Removes only the target directory (conservative approach)
- **Path Tracking**: Tracks created paths for potential future rollback improvements

**Example**:

```go
op := ops.NewCreateDir("project/data", 0755)
    .WithID("create-data-dir")
```

## Serializable Operations

### 3. **SerializableCreateFile Operation**

**Location**: `pkg/synthfs/serialization.go`

**Purpose**: JSON-serializable version of CreateFile for CLI tools and operation plans.

**Features**:

- Implements `SerializableOperation` interface
- JSON marshaling/unmarshaling support
- Used by the CLI tool for operation plans
- Same core functionality as regular CreateFile

## Core Operation Capabilities

All operations implement the `Operation` interface with these methods:

- **`ID()`** - Unique operation identifier
- **`Execute(ctx, fs)`** - Performs the actual filesystem change
- **`Validate(ctx, fs)`** - Validates the operation can be performed
- **`Dependencies()`** - Returns list of operation IDs this depends on
- **`Conflicts()`** - Returns conflicting operation IDs (currently returns nil)
- **`Rollback(ctx, fs)`** - Undoes the operation for transaction support
- **`Describe()`** - Returns human-readable operation description

## Planned Operations (Referenced but Not Implemented)

Based on the design documentation and code comments, these operations are planned:

### 4. **Copy Operation** (Planned)

- **Purpose**: Copy files/directories from source to destination
- **Referenced in**: Design docs mention `ops.Copy(src, dst string, opts ...CopyOption)`
- **Validation**: Would check source exists, destination is valid

### 5. **Move Operation** (Planned)  

- **Purpose**: Move/rename files and directories
- **Status**: Mentioned in design docs but not implemented

### 6. **Delete/Remove Operations** (Planned)

- **Purpose**: Delete files and directories
- **FileSystem Interface**: Already has `Remove()` and `RemoveAll()` methods
- **Status**: Infrastructure exists but operation wrappers not implemented

## Advanced Features

### **Dependency Management**

- Operations can declare dependencies on other operations
- Topological sorting ensures correct execution order
- Circular dependency detection

### **Conflict Detection**

- Framework exists for operations to declare conflicts
- Currently returns `nil` (no conflicts defined)
- Future enhancement for concurrent execution safety

### **Transaction Support**

- All operations support rollback for transaction-like behavior
- Rollback functions provided in execution results
- Conservative rollback approach (only undoes direct changes)

### **Progress Reporting**

- `ProgressReportingExecutor` for long-running operation batches
- Streaming execution with channels
- Console progress reporter included

### **CLI Integration**

- Operation plans can be serialized to JSON
- CLI commands: `plan create`, `plan execute`, `plan validate`
