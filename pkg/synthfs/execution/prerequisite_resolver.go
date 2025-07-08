package execution

import (
	"fmt"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// PrerequisiteResolver can create operations to satisfy prerequisites
type PrerequisiteResolver struct {
	factory core.OperationFactory
	logger  core.Logger
	idCounter int
}

// NewPrerequisiteResolver creates a new prerequisite resolver
func NewPrerequisiteResolver(factory core.OperationFactory, logger core.Logger) core.PrerequisiteResolver {
	return &PrerequisiteResolver{
		factory:   factory,
		logger:    logger,
		idCounter: 0,
	}
}

// CanResolve returns true if the resolver can resolve the given prerequisite
func (pr *PrerequisiteResolver) CanResolve(prereq core.Prerequisite) bool {
	// Only resolve parent directory prerequisites if we have a factory
	return prereq.Type() == "parent_dir" && pr.factory != nil
}

// Resolve creates operations to satisfy the given prerequisite
func (pr *PrerequisiteResolver) Resolve(prereq core.Prerequisite) ([]interface{}, error) {
	switch prereq.Type() {
	case "parent_dir":
		return pr.resolveParentDir(prereq)
	default:
		return nil, fmt.Errorf("unsupported prerequisite type: %s", prereq.Type())
	}
}

// resolveParentDir creates a CreateDirectory operation for parent directories
func (pr *PrerequisiteResolver) resolveParentDir(prereq core.Prerequisite) ([]interface{}, error) {
	if pr.factory == nil {
		return nil, fmt.Errorf("no operation factory available for prerequisite resolution")
	}

	path := prereq.Path()
	parentDir := filepath.Dir(path)

	// Skip if parent is root or current directory
	if parentDir == "." || parentDir == "/" || parentDir == path {
		pr.logger.Debug().
			Str("path", path).
			Str("parent", parentDir).
			Msg("skipping parent directory prerequisite - already at root")
		return []interface{}{}, nil
	}

	pr.logger.Debug().
		Str("path", path).
		Str("parent_dir", parentDir).
		Msg("resolving parent directory prerequisite")

	// Generate unique operation ID
	pr.idCounter++
	opID := core.OperationID(fmt.Sprintf("prereq_parent_dir_%d_%s", pr.idCounter, 
		filepath.Base(parentDir)))

	// Create parent directory operation
	op, err := pr.factory.CreateOperation(opID, "create_directory", parentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent directory operation: %w", err)
	}

	pr.logger.Debug().
		Str("op_id", string(opID)).
		Str("parent_dir", parentDir).
		Msg("created parent directory operation")

	return []interface{}{op}, nil
}