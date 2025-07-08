package batch

import (
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// This file contains legacy factory functions that are now provided by options.go
// Keeping for compatibility but delegating to options.go implementations

// NewBatchFactory creates a new batch factory
func NewBatchFactory() *Factory {
	return &Factory{}
}

// Factory creates batches with different implementations
type Factory struct{}

// CreateBatch creates a batch using the current default implementation
func (f *Factory) CreateBatch(fs interface{}, registry core.OperationFactory) Batch {
	opts := DefaultBatchOptions()
	return NewBatchWithOptions(fs, registry, opts)
}