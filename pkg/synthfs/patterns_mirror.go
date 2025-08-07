package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// MirrorOptions configures how a directory is mirrored with symlinks
type MirrorOptions struct {
	// Filter determines which files/dirs to include (return true to include)
	Filter func(path string, info fs.FileInfo) bool
	// IncludeDirectories creates directories instead of symlinking them
	IncludeDirectories bool
	// Overwrite existing symlinks
	Overwrite bool
}

// MirrorWithSymlinksOperation creates a directory structure with symlinks to original files
type MirrorWithSymlinksOperation struct {
	id      OperationID
	desc    OperationDesc
	srcDir  string
	dstDir  string
	options MirrorOptions
}

// NewMirrorWithSymlinksOperation creates a new mirror operation
func (s *SynthFS) NewMirrorWithSymlinksOperation(srcDir, dstDir string, opts ...MirrorOptions) *MirrorWithSymlinksOperation {
	var options MirrorOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	// Default filter accepts everything
	if options.Filter == nil {
		options.Filter = func(path string, info fs.FileInfo) bool { return true }
	}

	id := s.idGen("mirror_symlinks", srcDir)
	return &MirrorWithSymlinksOperation{
		id: id,
		desc: OperationDesc{
			Type: "mirror_symlinks",
			Path: srcDir,
			Details: map[string]interface{}{
				"src": srcDir,
				"dst": dstDir,
			},
		},
		srcDir:  srcDir,
		dstDir:  dstDir,
		options: options,
	}
}

// ID returns the operation ID
func (op *MirrorWithSymlinksOperation) ID() OperationID {
	return op.id
}

// Describe returns the operation description
func (op *MirrorWithSymlinksOperation) Describe() OperationDesc {
	return op.desc
}


// Prerequisites returns prerequisites for the operation
func (op *MirrorWithSymlinksOperation) Prerequisites() []core.Prerequisite {
	return []core.Prerequisite{
		core.NewSourceExistsPrerequisite(op.srcDir),
		core.NewParentDirPrerequisite(op.dstDir),
	}
}

// GetItem returns nil - no specific item
func (op *MirrorWithSymlinksOperation) GetItem() FsItem {
	return nil
}

// SetDescriptionDetail sets a detail in the description
func (op *MirrorWithSymlinksOperation) SetDescriptionDetail(key string, value interface{}) {
	if op.desc.Details == nil {
		op.desc.Details = make(map[string]interface{})
	}
	op.desc.Details[key] = value
}

// AddDependency adds a dependency
func (op *MirrorWithSymlinksOperation) AddDependency(depID OperationID) {
	// Not implemented for this operation
}

// SetPaths sets source and destination paths
func (op *MirrorWithSymlinksOperation) SetPaths(src, dst string) {
	op.srcDir = src
	op.dstDir = dst
	op.desc.Path = src
	op.desc.Details["src"] = src
	op.desc.Details["dst"] = dst
}

// GetChecksum returns nil
func (op *MirrorWithSymlinksOperation) GetChecksum(path string) *ChecksumRecord {
	return nil
}

// GetAllChecksums returns nil
func (op *MirrorWithSymlinksOperation) GetAllChecksums() map[string]*ChecksumRecord {
	return nil
}

// Execute is not implemented
func (op *MirrorWithSymlinksOperation) Execute(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return fmt.Errorf("Execute not implemented for MirrorWithSymlinksOperation")
}

// Validate is not implemented
func (op *MirrorWithSymlinksOperation) Validate(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return fmt.Errorf("Validate not implemented for MirrorWithSymlinksOperation")
}

// Rollback is not implemented yet
func (op *MirrorWithSymlinksOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	return fmt.Errorf("rollback not implemented for MirrorWithSymlinksOperation")
}

// ReverseOps generates reverse operations
func (op *MirrorWithSymlinksOperation) ReverseOps(ctx context.Context, fsys FileSystem, budget *core.BackupBudget) ([]Operation, *core.BackupData, error) {
	return nil, nil, fmt.Errorf("reverse ops not implemented for MirrorWithSymlinksOperation")
}



// MirrorBuilder provides a fluent interface for creating mirror operations
type MirrorBuilder struct {
	srcDir  string
	dstDir  string
	options MirrorOptions
}

// NewMirrorBuilder creates a new mirror builder
func NewMirrorBuilder(srcDir, dstDir string) *MirrorBuilder {
	return &MirrorBuilder{
		srcDir: srcDir,
		dstDir: dstDir,
		options: MirrorOptions{
			Filter: func(path string, info fs.FileInfo) bool { return true },
		},
	}
}

// WithFilter sets a filter function
func (b *MirrorBuilder) WithFilter(filter func(path string, info fs.FileInfo) bool) *MirrorBuilder {
	b.options.Filter = filter
	return b
}

// ExcludeHidden excludes hidden files (starting with .)
func (b *MirrorBuilder) ExcludeHidden() *MirrorBuilder {
	originalFilter := b.options.Filter
	b.options.Filter = func(path string, info fs.FileInfo) bool {
		name := filepath.Base(path)
		if strings.HasPrefix(name, ".") {
			return false
		}
		return originalFilter(path, info)
	}
	return b
}

// IncludeDirectories creates real directories instead of symlinking them
func (b *MirrorBuilder) IncludeDirectories() *MirrorBuilder {
	b.options.IncludeDirectories = true
	return b
}

// Overwrite enables overwriting existing symlinks
func (b *MirrorBuilder) Overwrite() *MirrorBuilder {
	b.options.Overwrite = true
	return b
}

// Build creates the mirror operation
func (b *MirrorBuilder) Build() Operation {
	return New().NewMirrorWithSymlinksOperation(b.srcDir, b.dstDir, b.options)
}

// Execute builds and executes the operation
func (b *MirrorBuilder) Execute(ctx context.Context, fs FileSystem) error {
	op := b.Build()
	return op.Execute(ctx, nil, fs)
}

// MirrorWithSymlinks is a convenience function
func MirrorWithSymlinks(ctx context.Context, fs FileSystem, srcDir, dstDir string) error {
	op := New().NewMirrorWithSymlinksOperation(srcDir, dstDir)
	return op.Execute(ctx, nil, fs)
}
