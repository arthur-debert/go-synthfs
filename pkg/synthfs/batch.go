package synthfs

import (
	"context"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Batch represents a collection of filesystem operations that can be validated and executed as a unit.
// This uses prerequisite resolution for automatic dependency management.
type Batch = batch.Batch

// NewBatch creates a new operation batch with default filesystem and context.
func NewBatch() *batch.BatchImpl {
	fs := filesystem.NewOSFileSystem(".")
	registry := GetDefaultRegistry()
	logger := NewLoggerAdapter(Logger())
	
	impl := batch.NewBatch(fs, registry).
		WithContext(context.Background()).
		WithLogger(logger)
	
	// Return the concrete type for direct access
	return impl.(*batch.BatchImpl)
}

// Result represents the outcome of executing a batch of operations
type Result = batch.Result

// ConvertBatchResult is no longer needed with the simplified design
// but kept for any remaining internal usage
func ConvertBatchResult(batchResult interface{}) *Result {
	if result, ok := batchResult.(batch.Result); ok {
		return &result
	}
	return nil
}