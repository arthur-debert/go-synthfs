package synthfs

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// SyncOptions configures how directories are synchronized
type SyncOptions struct {
	// DeleteExtra removes files in destination that don't exist in source
	DeleteExtra bool
	// UpdateNewer only updates files if source is newer
	UpdateNewer bool
	// PreserveSymlinks copies symlinks as symlinks (not their targets)
	PreserveSymlinks bool
	// Filter determines which files to sync (return true to include)
	Filter func(path string, info fs.FileInfo) bool
	// DryRun reports what would be done without making changes
	DryRun bool
}

// SyncResult contains information about a sync operation
type SyncResult struct {
	FilesCreated    []string
	FilesUpdated    []string
	FilesDeleted    []string
	DirsCreated     []string
	DirsDeleted     []string
	SymlinksCreated []string
	Errors          []error
}

// SyncOperation synchronizes two directories
type SyncOperation struct {
	id      OperationID
	desc    OperationDesc
	srcDir  string
	dstDir  string
	options SyncOptions
	result  *SyncResult
}

// NewSyncOperation creates a new sync operation
func (s *SynthFS) NewSyncOperation(srcDir, dstDir string, opts ...SyncOptions) *SyncOperation {
	var options SyncOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	// Default filter accepts everything
	if options.Filter == nil {
		options.Filter = func(path string, info fs.FileInfo) bool { return true }
	}

	id := s.idGen("sync", srcDir)
	return &SyncOperation{
		id: id,
		desc: OperationDesc{
			Type: "sync",
			Path: srcDir,
			Details: map[string]interface{}{
				"src":          srcDir,
				"dst":          dstDir,
				"delete_extra": options.DeleteExtra,
				"update_newer": options.UpdateNewer,
				"dry_run":      options.DryRun,
			},
		},
		srcDir:  srcDir,
		dstDir:  dstDir,
		options: options,
		result:  &SyncResult{},
	}
}

// GetResult returns the sync result after execution
func (op *SyncOperation) GetResult() *SyncResult {
	return op.result
}

// ID returns the operation ID
func (op *SyncOperation) ID() OperationID {
	return op.id
}

// Describe returns the operation description
func (op *SyncOperation) Describe() OperationDesc {
	return op.desc
}


// Prerequisites returns prerequisites for the operation
func (op *SyncOperation) Prerequisites() []core.Prerequisite {
	return []core.Prerequisite{
		core.NewSourceExistsPrerequisite(op.srcDir),
	}
}

// GetItem returns nil - no specific item
func (op *SyncOperation) GetItem() FsItem {
	return nil
}

// SetDescriptionDetail sets a detail in the description
func (op *SyncOperation) SetDescriptionDetail(key string, value interface{}) {
	if op.desc.Details == nil {
		op.desc.Details = make(map[string]interface{})
	}
	op.desc.Details[key] = value
}

// AddDependency adds a dependency
func (op *SyncOperation) AddDependency(depID OperationID) {
	// Not implemented for this operation
}

// SetPaths sets source and destination paths
func (op *SyncOperation) SetPaths(src, dst string) {
	op.srcDir = src
	op.dstDir = dst
	op.desc.Path = src
	op.desc.Details["src"] = src
	op.desc.Details["dst"] = dst
}

// GetChecksum returns nil
func (op *SyncOperation) GetChecksum(path string) *ChecksumRecord {
	return nil
}

// GetAllChecksums returns nil
func (op *SyncOperation) GetAllChecksums() map[string]*ChecksumRecord {
	return nil
}

// Execute is not implemented
func (op *SyncOperation) Execute(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return fmt.Errorf("Execute not implemented for SyncOperation")
}

// Validate is not implemented
func (op *SyncOperation) Validate(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return fmt.Errorf("Validate not implemented for SyncOperation")
}

// Rollback is not implemented yet
func (op *SyncOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	return fmt.Errorf("rollback not implemented for SyncOperation")
}

// ReverseOps generates reverse operations
func (op *SyncOperation) ReverseOps(ctx context.Context, fsys FileSystem, budget *core.BackupBudget) ([]Operation, *core.BackupData, error) {
	return nil, nil, fmt.Errorf("reverse ops not implemented for SyncOperation")
}




// Sync creates a sync operation
func (s *SynthFS) Sync(srcDir, dstDir string, opts ...SyncOptions) Operation {
	return s.NewSyncOperation(srcDir, dstDir, opts...)
}

// SyncDirectories is a convenience function that syncs directories directly
func SyncDirectories(ctx context.Context, fs FileSystem, srcDir, dstDir string, opts ...SyncOptions) (*SyncResult, error) {
	op := New().NewSyncOperation(srcDir, dstDir, opts...)
	err := op.Execute(ctx, nil, fs)
	return op.GetResult(), err
}

// SyncBuilder provides a fluent interface for sync operations
type SyncBuilder struct {
	srcDir  string
	dstDir  string
	options SyncOptions
}

// NewSyncBuilder creates a new sync builder
func NewSyncBuilder(srcDir, dstDir string) *SyncBuilder {
	return &SyncBuilder{
		srcDir: srcDir,
		dstDir: dstDir,
		options: SyncOptions{
			Filter: func(path string, info fs.FileInfo) bool { return true },
		},
	}
}

// DeleteExtra enables deletion of extra files in destination
func (sb *SyncBuilder) DeleteExtra() *SyncBuilder {
	sb.options.DeleteExtra = true
	return sb
}

// UpdateNewer only updates files if source is newer
func (sb *SyncBuilder) UpdateNewer() *SyncBuilder {
	sb.options.UpdateNewer = true
	return sb
}

// PreserveSymlinks preserves symlinks as symlinks
func (sb *SyncBuilder) PreserveSymlinks() *SyncBuilder {
	sb.options.PreserveSymlinks = true
	return sb
}

// WithFilter sets a filter function
func (sb *SyncBuilder) WithFilter(filter func(path string, info fs.FileInfo) bool) *SyncBuilder {
	sb.options.Filter = filter
	return sb
}

// DryRun enables dry run mode
func (sb *SyncBuilder) DryRun() *SyncBuilder {
	sb.options.DryRun = true
	return sb
}

// Build creates the sync operation
func (sb *SyncBuilder) Build() Operation {
	return New().NewSyncOperation(sb.srcDir, sb.dstDir, sb.options)
}

// Execute builds and executes the operation
func (sb *SyncBuilder) Execute(ctx context.Context, fs FileSystem) (*SyncResult, error) {
	op := sb.Build().(*SyncOperation)
	err := op.Execute(ctx, nil, fs)
	return op.GetResult(), err
}

