package batch

import (
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// NewBatchWithOptions creates a new batch with specified options, allowing migration between old and new implementations
func NewBatchWithOptions(fs interface{}, registry core.OperationFactory, opts BatchOptions) Batch {
	if opts.UseSimpleBatch {
		// Use the new SimpleBatch implementation with prerequisite resolution
		return NewSimpleBatch(fs, registry)
	} else {
		// Use the existing BatchImpl implementation
		return NewBatch(fs, registry)
	}
}

// DefaultBatchOptions returns the default options for backward compatibility
func DefaultBatchOptions() BatchOptions {
	return BatchOptions{
		UseSimpleBatch: false, // Default to existing behavior for backward compatibility
	}
}