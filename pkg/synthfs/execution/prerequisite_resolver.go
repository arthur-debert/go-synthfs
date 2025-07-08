package execution

import (
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// PrerequisiteResolver resolves prerequisites by creating necessary operations
type PrerequisiteResolver struct {
	operationFactory core.OperationFactory
	idCounter        int
	logger           core.Logger
}

// NewPrerequisiteResolver creates a new prerequisite resolver
func NewPrerequisiteResolver(factory core.OperationFactory, logger core.Logger) *PrerequisiteResolver {
	return &PrerequisiteResolver{
		operationFactory: factory,
		idCounter:        0,
		logger:           logger,
	}
}

// CanResolve checks if a prerequisite can be resolved
func (pr *PrerequisiteResolver) CanResolve(prereq core.Prerequisite) bool {
	switch prereq.Type() {
	case "parent_dir":
		return true
	case "no_conflict":
		return false // Cannot auto-resolve conflicts
	case "source_exists":
		return false // Cannot auto-create sources
	default:
		return false
	}
}

// Resolve creates operations to satisfy the prerequisite
func (pr *PrerequisiteResolver) Resolve(prereq core.Prerequisite) ([]interface{}, error) {
	switch prereq.Type() {
	case "parent_dir":
		return pr.resolveParentDir(prereq)
	default:
		return nil, fmt.Errorf("cannot resolve prerequisite type: %s", prereq.Type())
	}
}

// resolveParentDir creates parent directory operations
func (pr *PrerequisiteResolver) resolveParentDir(prereq core.Prerequisite) ([]interface{}, error) {
	path := prereq.Path()
	if path == "" {
		return nil, fmt.Errorf("parent_dir prerequisite has empty path")
	}

	// Get parent directory path
	parentPath := getParentPath(path)
	if parentPath == "." || parentPath == "/" {
		return nil, nil // No parent directory needed
	}

	// Generate unique operation ID
	pr.idCounter++
	opID := core.OperationID(fmt.Sprintf("prereq_parent_dir_%d_%s", pr.idCounter, cleanPath(parentPath)))

	// Create parent directory operation
	op, err := pr.operationFactory.CreateOperation(opID, "create_directory", parentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent directory operation: %w", err)
	}

	// Set default directory item
	if err := pr.setDefaultDirItem(op, parentPath); err != nil {
		return nil, fmt.Errorf("failed to set directory item: %w", err)
	}

	return []interface{}{op}, nil
}

// setDefaultDirItem sets a default directory item for the operation
func (pr *PrerequisiteResolver) setDefaultDirItem(op interface{}, path string) error {
	// Create a minimal directory item
	dirItem := &defaultDirItem{
		path: path,
		mode: 0755, // Default directory permissions
	}

	return pr.operationFactory.SetItemForOperation(op, dirItem)
}

// Helper functions

// getParentPath extracts the parent directory path
func getParentPath(path string) string {
	if path == "" || path == "." || path == "/" {
		return ""
	}
	
	// Simple parent path extraction (could use filepath.Dir)
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			parent := path[:i]
			if parent == "" {
				return "/"
			}
			return parent
		}
	}
	return "."
}

// cleanPath removes invalid characters from paths for IDs
func cleanPath(path string) string {
	result := ""
	for _, char := range path {
		if char == '/' || char == '\\' {
			result += "_"
		} else if char == ':' {
			result += "_"
		} else {
			result += string(char)
		}
	}
	return result
}

// defaultDirItem is a minimal directory item implementation
type defaultDirItem struct {
	path string
	mode int
}

func (d *defaultDirItem) Path() string {
	return d.path
}

func (d *defaultDirItem) Type() string {
	return "directory"
}

func (d *defaultDirItem) Mode() interface{} {
	return d.mode
}

func (d *defaultDirItem) IsDir() bool {
	return true
}