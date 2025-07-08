package execution

import (
	"fmt"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// PrerequisiteResolver resolves prerequisites into operations
type PrerequisiteResolver struct {
	factory core.OperationFactory
}

// NewPrerequisiteResolver creates a new prerequisite resolver
func NewPrerequisiteResolver(factory core.OperationFactory, logger core.Logger) *PrerequisiteResolver {
	return &PrerequisiteResolver{
		factory: factory,
	}
}

// CanResolve checks if this resolver can handle a prerequisite
func (r *PrerequisiteResolver) CanResolve(prereq core.Prerequisite) bool {
	switch prereq.Type() {
	case "parent_dir":
		return true
	case "no_conflict", "source_exists":
		// These are validation-only prerequisites, not resolvable
		return false
	default:
		return false
	}
}

// Resolve creates operations to satisfy a prerequisite
func (r *PrerequisiteResolver) Resolve(prereq core.Prerequisite) ([]interface{}, error) {
	switch prereq.Type() {
	case "parent_dir":
		return r.resolveParentDir(prereq)
	default:
		return nil, fmt.Errorf("cannot resolve prerequisite type: %s", prereq.Type())
	}
}

// resolveParentDir creates a directory creation operation
func (r *PrerequisiteResolver) resolveParentDir(prereq core.Prerequisite) ([]interface{}, error) {
	parentPath := prereq.Path()
	if parentPath == "." || parentPath == "/" {
		// Root or current directory doesn't need creation
		return nil, nil
	}

	// Generate a unique operation ID
	opID := core.OperationID(fmt.Sprintf("prereq_parent_dir_%s", parentPath))

	// Create directory operation using the factory
	dirOp, err := r.factory.CreateOperation(opID, "create_directory", parentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent directory operation: %w", err)
	}

	// Create a minimal directory item
	dirItem := &PrereqDirectoryItem{
		path: parentPath,
		mode: fs.FileMode(0755), // Default directory permissions
	}

	// Set the item on the operation
	if err := r.factory.SetItemForOperation(dirOp, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set directory item: %w", err)
	}

	return []interface{}{dirOp}, nil
}

// PrereqDirectoryItem is a minimal directory item for prerequisite resolution
type PrereqDirectoryItem struct {
	path string
	mode fs.FileMode
}

func (d *PrereqDirectoryItem) Path() string      { return d.path }
func (d *PrereqDirectoryItem) Type() string      { return "directory" }
func (d *PrereqDirectoryItem) Mode() fs.FileMode { return d.mode }
func (d *PrereqDirectoryItem) IsDir() bool       { return true }