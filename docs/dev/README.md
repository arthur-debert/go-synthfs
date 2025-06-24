# go-synthfs Developer Guide

## Logging

go-synthfs uses [zerolog](https://github.com/rs/zerolog) for structured logging with a centralized configuration system.

### Basic Usage

```go
import "github.com/arthur-debert/synthfs/pkg/synthfs"

// Set log level for the entire library
synthfs.SetLogLevel(zerolog.InfoLevel)

// Or use string-based configuration
synthfs.SetLogLevelFromString("debug")

// Get current log level
level := synthfs.GetLogLevel()
```

### Log Levels

- **Disabled**: No logging
- **Panic**: Only panic-level events  
- **Fatal**: Fatal errors that cause program termination
- **Error**: Error conditions
- **Warn**: Warning conditions (default)
- **Info**: Informational messages about operation flow
- **Debug**: Detailed information for debugging, including branching logic
- **Trace**: Very detailed tracing information with full data dumps

### Testing Configuration

Configure logging verbosity in tests using the `-v` flags:

```go
func TestMain(m *testing.M) {
    // Parse test verbosity and configure logging
    verbose := 0
    for _, arg := range os.Args {
        if arg == "-v" {
            verbose = 1
        } else if arg == "-vv" {
            verbose = 2  
        } else if arg == "-vvv" {
            verbose = 3
        }
    }
    
    synthfs.SetupTestLogging(verbose)
    
    os.Exit(m.Run())
}
```

**Test verbosity mapping:**

- No flags: `Warn` level (quiet)
- `-v`: `Info` level (operation flow)
- `-vv`: `Debug` level (detailed debugging)
- `-vvv`: `Trace` level (full tracing)

### Application Integration

Applications using go-synthfs can control library logging:

```go
// Disable synthfs logging entirely
synthfs.DisableLogging()

// Set to info level to see operation flow
synthfs.SetLogLevel(zerolog.InfoLevel)

// Redirect synthfs logs to a file
logFile, _ := os.Create("synthfs.log")
synthfs.SetLogOutput(logFile)
```

### Log Output Format

All synthfs logs include a `lib=synthfs` field to identify library messages:

```
2024-01-15T10:30:45Z INF operation started lib=synthfs op_id=create-file-001 op_type=CreateFile path=test.txt
2024-01-15T10:30:45Z INF operation completed successfully lib=synthfs op_id=create-file-001 op_type=CreateFile path=test.txt success=true duration=1.2ms
```

### Built-in Logging Functions

The library provides convenience functions for common logging patterns:

```go
// Log operation lifecycle
synthfs.LogOperationStart(opID, "CreateFile", "/path/to/file")
synthfs.LogOperationComplete(opID, "CreateFile", "/path/to/file", true, duration)

// Log validation results  
synthfs.LogValidationResult(opID, "CreateFile", "/path/to/file", false, "file already exists")

// Get the global logger for custom logging
logger := synthfs.Logger()
logger.Debug().Str("key", "value").Msg("custom debug message")
```

## Coverage Exclusions

Test utilities and mock implementations are excluded from code coverage using Go build tags to avoid skewing coverage metrics:

```go
//go:build !coverage

package testutil
```

**Excluded files:**

- `pkg/synthfs/testutil/` - Mock filesystem implementations for testing
- `pkg/synthfs/testing.go` - Test helpers and utilities

**Usage:** Coverage scripts automatically use `-tags=coverage` to exclude these files. No developer action required.
