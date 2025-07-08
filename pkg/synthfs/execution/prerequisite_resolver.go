package execution

import (
	"fmt"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// PrerequisiteResolver implements core.PrerequisiteResolver
type PrerequisiteResolver struct {
	operationFactory core.OperationFactory
	logger           core.Logger
}

// NewPrerequisiteResolver creates a new prerequisite resolver
func NewPrerequisiteResolver(operationFactory core.OperationFactory, logger core.Logger) *PrerequisiteResolver {
	if logger == nil {
		logger = &noOpLogger{}
	}
	return &PrerequisiteResolver{
		operationFactory: operationFactory,
		logger:           logger,
	}
}

// CanResolve returns true if this resolver can create operations to satisfy the prerequisite
func (pr *PrerequisiteResolver) CanResolve(prereq core.Prerequisite) bool {
	switch prereq.Type() {
	case "parent_dir":
		return true
	case "no_conflict":
		return false // This is a validation prerequisite, not resolvable
	case "source_exists":
		return false // This is a validation prerequisite, not resolvable
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
		return nil, fmt.Errorf("cannot resolve prerequisite of type: %s", prereq.Type())
	}
}

// resolveParentDir creates directory creation operations for parent directories
func (pr *PrerequisiteResolver) resolveParentDir(prereq core.Prerequisite) ([]interface{}, error) {
	path := prereq.Path()
	parentDir := filepath.Dir(path)

	// If parent is root or current directory, no operation needed
	if parentDir == "." || parentDir == "/" || parentDir == path {
		return []interface{}{}, nil
	}

	pr.logger.Debug().
		Str("path", path).
		Str("parent_dir", parentDir).
		Msg("resolving parent directory prerequisite")

	// Check if we have an operation factory
	if pr.operationFactory == nil {
		return nil, fmt.Errorf("cannot resolve parent directory prerequisite: no operation factory provided")
	}

	// Create a directory creation operation for the parent
	opID := core.OperationID(fmt.Sprintf("prereq_parent_dir_%s", filepath.Base(parentDir)))
	op, err := pr.operationFactory.CreateOperation(opID, "create_directory", parentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create parent directory operation: %w", err)
	}

	// Set a default directory item if the factory supports it
	if itemSetter, ok := op.(interface{ SetItem(interface{}) }); ok {
		// Create a minimal directory item
		dirItem := &minimalDirItem{
			path: parentDir,
			mode: 0755,
		}
		itemSetter.SetItem(dirItem)
	}

	pr.logger.Debug().
		Str("op_id", string(opID)).
		Str("parent_dir", parentDir).
		Msg("created parent directory operation")

	return []interface{}{op}, nil
}

// minimalDirItem is a minimal directory item for prerequisites
type minimalDirItem struct {
	path string
	mode int
}

func (m *minimalDirItem) Path() string      { return m.path }
func (m *minimalDirItem) Type() string      { return "directory" }
func (m *minimalDirItem) Mode() interface{} { return m.mode }
func (m *minimalDirItem) IsDir() bool       { return true }

// noOpLogger implements core.Logger for when no logger is provided
type noOpLogger struct{}

func (l *noOpLogger) Trace() core.LogEvent { return &noOpLogEvent{} }
func (l *noOpLogger) Debug() core.LogEvent { return &noOpLogEvent{} }
func (l *noOpLogger) Info() core.LogEvent  { return &noOpLogEvent{} }
func (l *noOpLogger) Warn() core.LogEvent  { return &noOpLogEvent{} }
func (l *noOpLogger) Error() core.LogEvent { return &noOpLogEvent{} }

// noOpLogEvent implements core.LogEvent with no-op methods
type noOpLogEvent struct{}

func (e *noOpLogEvent) Str(key, val string) core.LogEvent             { return e }
func (e *noOpLogEvent) Int(key string, val int) core.LogEvent         { return e }
func (e *noOpLogEvent) Bool(key string, val bool) core.LogEvent       { return e }
func (e *noOpLogEvent) Dur(key string, val interface{}) core.LogEvent { return e }
func (e *noOpLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *noOpLogEvent) Err(err error) core.LogEvent                   { return e }
func (e *noOpLogEvent) Float64(key string, val float64) core.LogEvent { return e }
func (e *noOpLogEvent) Msg(msg string)                                {}