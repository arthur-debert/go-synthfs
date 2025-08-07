package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// CopyTreeOptions configures how a directory tree is copied
type CopyTreeOptions struct {
	// Filter determines which files/dirs to include (return true to include)
	Filter func(path string, info fs.FileInfo) bool
	// PreservePermissions copies file permissions
	PreservePermissions bool
	// PreserveTimestamps copies modification times
	PreserveTimestamps bool
	// FollowSymlinks follows symlinks instead of copying them
	FollowSymlinks bool
	// Overwrite existing files
	Overwrite bool
}

// CopyTree creates operations to recursively copy a directory tree
func (s *SynthFS) CopyTree(srcDir, dstDir string, opts ...CopyTreeOptions) ([]Operation, error) {
	var options CopyTreeOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	// Default filter accepts everything
	if options.Filter == nil {
		options.Filter = func(path string, info fs.FileInfo) bool { return true }
	}

	var ops []Operation

	// Walk the source directory and create copy operations
	_ = func(srcPath string, dstPath string, info fs.FileInfo) error {
		// Apply filter
		if !options.Filter(srcPath, info) {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			// Create directory
			op := s.CreateDir(targetPath, info.Mode())
			ops = append(ops, op)
		} else if info.Mode()&fs.ModeSymlink != 0 && !options.FollowSymlinks {
			// Copy symlink
			op := s.CreateSymlink("", targetPath) // We'd need to read the link target
			ops = append(ops, op)
		} else {
			// Copy file
			op := s.Copy(srcPath, targetPath)
			if options.PreservePermissions {
				if adapter, ok := op.(*OperationsPackageAdapter); ok {
					adapter.SetDescriptionDetail("preserve_mode", true)
					adapter.SetDescriptionDetail("mode", info.Mode())
				}
			}
			ops = append(ops, op)
		}

		return nil
	}

	// Since we can't actually walk the filesystem here (no fs parameter),
	// we'll return a function that creates the operations when executed
	// For now, we'll create a custom operation that handles this
	return nil, fmt.Errorf("CopyTree requires filesystem access - use CopyTreeOperation instead")
}

// CopyTreeOperation is a special operation that copies an entire directory tree
type CopyTreeOperation struct {
	id      OperationID
	desc    OperationDesc
	srcDir  string
	dstDir  string
	options CopyTreeOptions
}

// NewCopyTreeOperation creates a new copy tree operation
func (s *SynthFS) NewCopyTreeOperation(srcDir, dstDir string, opts ...CopyTreeOptions) *CopyTreeOperation {
	var options CopyTreeOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	// Default filter accepts everything
	if options.Filter == nil {
		options.Filter = func(path string, info fs.FileInfo) bool { return true }
	}

	id := s.idGen("copy_tree", srcDir)
	return &CopyTreeOperation{
		id: id,
		desc: OperationDesc{
			Type: "copy_tree",
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
func (op *CopyTreeOperation) ID() OperationID {
	return op.id
}

// Describe returns the operation description
func (op *CopyTreeOperation) Describe() OperationDesc {
	return op.desc
}


// Prerequisites returns prerequisites for the operation
func (op *CopyTreeOperation) Prerequisites() []core.Prerequisite {
	return []core.Prerequisite{
		core.NewSourceExistsPrerequisite(op.srcDir),
		core.NewParentDirPrerequisite(op.dstDir),
	}
}

// GetItem returns nil - no specific item
func (op *CopyTreeOperation) GetItem() FsItem {
	return nil
}

// SetDescriptionDetail sets a detail in the description
func (op *CopyTreeOperation) SetDescriptionDetail(key string, value interface{}) {
	if op.desc.Details == nil {
		op.desc.Details = make(map[string]interface{})
	}
	op.desc.Details[key] = value
}

// AddDependency adds a dependency
func (op *CopyTreeOperation) AddDependency(depID OperationID) {
	// Not implemented for this operation
}

// SetPaths sets source and destination paths
func (op *CopyTreeOperation) SetPaths(src, dst string) {
	op.srcDir = src
	op.dstDir = dst
	op.desc.Path = src
	op.desc.Details["src"] = src
	op.desc.Details["dst"] = dst
}

// GetChecksum returns nil
func (op *CopyTreeOperation) GetChecksum(path string) *ChecksumRecord {
	return nil
}

// GetAllChecksums returns nil
func (op *CopyTreeOperation) GetAllChecksums() map[string]*ChecksumRecord {
	return nil
}

// Execute is not implemented
func (op *CopyTreeOperation) Execute(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return fmt.Errorf("Execute not implemented for CopyTreeOperation")
}

// Validate is not implemented
func (op *CopyTreeOperation) Validate(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return fmt.Errorf("Validate not implemented for CopyTreeOperation")
}

// Rollback is not implemented yet
func (op *CopyTreeOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	// Would need to track all created files/dirs
	return fmt.Errorf("rollback not implemented for CopyTreeOperation")
}

// ReverseOps generates reverse operations
func (op *CopyTreeOperation) ReverseOps(ctx context.Context, fsys FileSystem, budget *core.BackupBudget) ([]Operation, *core.BackupData, error) {
	// Would create a delete operation for the destination
	return nil, nil, fmt.Errorf("reverse ops not implemented for CopyTreeOperation")
}



// CopyTreeBuilder provides a fluent interface for configuring tree copies
type CopyTreeBuilder struct {
	srcDir  string
	dstDir  string
	options CopyTreeOptions
}

// NewCopyTreeBuilder creates a new copy tree builder
func NewCopyTreeBuilder(srcDir, dstDir string) *CopyTreeBuilder {
	return &CopyTreeBuilder{
		srcDir: srcDir,
		dstDir: dstDir,
		options: CopyTreeOptions{
			Filter: func(path string, info fs.FileInfo) bool { return true },
		},
	}
}

// WithFilter sets a filter function
func (b *CopyTreeBuilder) WithFilter(filter func(path string, info fs.FileInfo) bool) *CopyTreeBuilder {
	b.options.Filter = filter
	return b
}

// ExcludeHidden excludes hidden files (starting with .)
func (b *CopyTreeBuilder) ExcludeHidden() *CopyTreeBuilder {
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

// ExcludePattern excludes files matching the pattern
func (b *CopyTreeBuilder) ExcludePattern(pattern string) *CopyTreeBuilder {
	originalFilter := b.options.Filter
	b.options.Filter = func(path string, info fs.FileInfo) bool {
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return false
		}
		return originalFilter(path, info)
	}
	return b
}

// PreservePermissions enables permission preservation
func (b *CopyTreeBuilder) PreservePermissions() *CopyTreeBuilder {
	b.options.PreservePermissions = true
	return b
}

// PreserveTimestamps enables timestamp preservation
func (b *CopyTreeBuilder) PreserveTimestamps() *CopyTreeBuilder {
	b.options.PreserveTimestamps = true
	return b
}

// FollowSymlinks enables following symlinks
func (b *CopyTreeBuilder) FollowSymlinks() *CopyTreeBuilder {
	b.options.FollowSymlinks = true
	return b
}

// Overwrite enables overwriting existing files
func (b *CopyTreeBuilder) Overwrite() *CopyTreeBuilder {
	b.options.Overwrite = true
	return b
}

// Build creates the copy tree operation
func (b *CopyTreeBuilder) Build() Operation {
	return New().NewCopyTreeOperation(b.srcDir, b.dstDir, b.options)
}

// Execute builds and executes the operation
func (b *CopyTreeBuilder) Execute(ctx context.Context, fs FileSystem) error {
	op := b.Build()
	return op.Execute(ctx, nil, fs)
}

// CopyTreeFunc is a convenience function for copying directory trees
func CopyTreeFunc(ctx context.Context, fs FileSystem, srcDir, dstDir string) error {
	op := New().NewCopyTreeOperation(srcDir, dstDir)
	return op.Execute(ctx, nil, fs)
}
