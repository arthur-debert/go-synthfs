package batch

import (
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// BatchOptions holds configuration options for batch execution
type BatchOptions struct {
	// Execution options
	Restorable           bool
	MaxBackupSizeMB      int
	ResolvePrerequisites bool
}

// DefaultBatchOptions returns the default batch options
func DefaultBatchOptions() *BatchOptions {
	return &BatchOptions{
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: true, // Always enable prerequisite resolution
	}
}

// WithSimpleBatch is deprecated and no-op. All batches now use prerequisite resolution.
func (opts *BatchOptions) WithSimpleBatch(enabled bool) *BatchOptions {
	// No-op for backward compatibility
	return opts
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

	// Phase 7: Always use the unified batch implementation with prerequisite resolution
	return NewBatch(fs, registry)
}