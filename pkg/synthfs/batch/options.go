package batch

import (
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// BatchOptions holds configuration options for batch execution
type BatchOptions struct {
	// Implementation selection (Phase 6: defaults to true)
	UseSimpleBatch       bool // When true, use SimpleBatch + prerequisite resolution (default: true in Phase 6)
	
	// Execution options
	Restorable           bool
	MaxBackupSizeMB      int
	ResolvePrerequisites bool
}

// DefaultBatchOptions returns the default batch options (Phase 6: SimpleBatch is default)
func DefaultBatchOptions() *BatchOptions {
	return &BatchOptions{
		UseSimpleBatch:       true,  // Phase 6: Default to SimpleBatch
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: true, // Always enable prerequisite resolution
	}
}

// WithSimpleBatch configures whether to use SimpleBatch implementation
func (opts *BatchOptions) WithSimpleBatch(enabled bool) *BatchOptions {
	newOpts := *opts
	newOpts.UseSimpleBatch = enabled
	return &newOpts
}

// WithRestorable configures backup/restore options
func (opts *BatchOptions) WithRestorable(enabled bool) *BatchOptions {
	newOpts := *opts
	newOpts.Restorable = enabled
	return &newOpts
}

// WithBackupBudget sets the backup budget size
func (opts *BatchOptions) WithBackupBudget(maxMB int) *BatchOptions {
	newOpts := *opts
	newOpts.MaxBackupSizeMB = maxMB
	return &newOpts
}

// WithPrerequisiteResolution enables/disables prerequisite resolution
func (opts *BatchOptions) WithPrerequisiteResolution(enabled bool) *BatchOptions {
	newOpts := *opts
	newOpts.ResolvePrerequisites = enabled
	return &newOpts
}

// ToPipelineOptions converts BatchOptions to core.PipelineOptions
func (opts *BatchOptions) ToPipelineOptions() core.PipelineOptions {
	return core.PipelineOptions{
		Restorable:           opts.Restorable,
		MaxBackupSizeMB:      opts.MaxBackupSizeMB,
		ResolvePrerequisites: opts.ResolvePrerequisites,
	}
}

// ToRunOptions converts BatchOptions to a map for RunWithOptions
func (opts *BatchOptions) ToRunOptions() map[string]interface{} {
	return map[string]interface{}{
		"use_simple_batch":      opts.UseSimpleBatch,
		"restorable":            opts.Restorable,
		"max_backup_size_mb":    opts.MaxBackupSizeMB,
		"resolve_prerequisites": opts.ResolvePrerequisites,
	}
}

// NewBatchWithOptions creates a batch using the specified options
func NewBatchWithOptions(fs interface{}, registry core.OperationFactory, opts *BatchOptions) Batch {
	if opts == nil {
		opts = DefaultBatchOptions()
	}

	// Phase 6: Use SimpleBatch when UseSimpleBatch is true (default), otherwise use legacy
	if opts.UseSimpleBatch {
		return NewSimpleBatch(fs, registry)
	} else {
		return NewBatch(fs, registry)
	}
}