package batch

import (
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// BatchOptions holds configuration options for batch creation and execution
type BatchOptions struct {
	// UseSimpleBatch determines which batch implementation to use
	// When true: Use SimpleBatch + prerequisite resolution
	// When false (default): Use existing behavior with auto parent dir creation
	UseSimpleBatch bool

	// Execution options
	Restorable           bool
	MaxBackupSizeMB      int
	ResolvePrerequisites bool

	// Advanced options for legacy mode
	AutoCreateParentDirs bool // Only used when UseSimpleBatch=false
}

// DefaultBatchOptions returns the default batch options
func DefaultBatchOptions() *BatchOptions {
	return &BatchOptions{
		UseSimpleBatch:       false, // Default to existing behavior
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: false, // Default to legacy mode for compatibility
		AutoCreateParentDirs: true,  // Legacy mode auto-creates parent dirs
	}
}

// SimpleBatchOptions returns batch options configured for SimpleBatch usage
func SimpleBatchOptions() *BatchOptions {
	return &BatchOptions{
		UseSimpleBatch:       true,
		Restorable:           false,
		MaxBackupSizeMB:      10,
		ResolvePrerequisites: true, // SimpleBatch always uses prerequisite resolution
		AutoCreateParentDirs: false, // SimpleBatch doesn't auto-create parent dirs
	}
}

// WithSimpleBatch configures options to use SimpleBatch implementation
func (opts *BatchOptions) WithSimpleBatch(enabled bool) *BatchOptions {
	newOpts := *opts
	newOpts.UseSimpleBatch = enabled
	if enabled {
		// When using SimpleBatch, always enable prerequisite resolution
		newOpts.ResolvePrerequisites = true
		newOpts.AutoCreateParentDirs = false
	} else {
		// When using traditional batch, use legacy defaults
		newOpts.ResolvePrerequisites = false
		newOpts.AutoCreateParentDirs = true
	}
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
		"restorable":            opts.Restorable,
		"max_backup_size_mb":    opts.MaxBackupSizeMB,
		"resolve_prerequisites": opts.ResolvePrerequisites,
		"use_simple_batch":      opts.UseSimpleBatch,
		"auto_create_parents":   opts.AutoCreateParentDirs,
	}
}

// NewBatchWithOptions creates a batch using the specified options
func NewBatchWithOptions(fs interface{}, registry core.OperationFactory, opts *BatchOptions) Batch {
	if opts == nil {
		opts = DefaultBatchOptions()
	}

	if opts.UseSimpleBatch {
		return NewSimpleBatch(fs, registry)
	} else {
		return NewBatch(fs, registry)
	}
}