package synthfs

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
)

// SerializableOperation extends Operation with JSON serialization capabilities
type SerializableOperation interface {
	Operation
	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

// OperationPlan represents a serializable plan of operations
type OperationPlan struct {
	Operations []SerializableOperation `json:"operations"`
	Metadata   PlanMetadata            `json:"metadata"`
}

// PlanMetadata contains information about the operation plan
type PlanMetadata struct {
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// SerializableOperationData represents the JSON structure for operations
type SerializableOperationData struct {
	Type         string                 `json:"type"`
	ID           string                 `json:"id"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Conflicts    []string               `json:"conflicts,omitempty"`
	Parameters   map[string]interface{} `json:"parameters"`
}

// MarshalPlan serializes an operation plan to JSON
func MarshalPlan(plan *OperationPlan) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}

// UnmarshalPlan deserializes an operation plan from JSON
func UnmarshalPlan(data []byte) (*OperationPlan, error) {
	// First unmarshal into a raw structure to handle the operations manually
	var rawPlan struct {
		Operations []json.RawMessage `json:"operations"`
		Metadata   PlanMetadata      `json:"metadata"`
	}

	if err := json.Unmarshal(data, &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation plan: %w", err)
	}

	plan := &OperationPlan{
		Operations: make([]SerializableOperation, 0, len(rawPlan.Operations)),
		Metadata:   rawPlan.Metadata,
	}

	// Unmarshal each operation based on its type
	for _, rawOp := range rawPlan.Operations {
		var opType struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(rawOp, &opType); err != nil {
			return nil, fmt.Errorf("failed to determine operation type: %w", err)
		}

		switch opType.Type {
		case "create_file":
			var op SerializableCreateFileOperation
			if err := op.UnmarshalJSON(rawOp); err != nil {
				return nil, fmt.Errorf("failed to unmarshal create_file operation: %w", err)
			}
			plan.Operations = append(plan.Operations, &op)
		default:
			return nil, fmt.Errorf("unknown operation type: %s", opType.Type)
		}
	}

	return plan, nil
}

// NewOperationPlan creates a new operation plan
func NewOperationPlan(description string) *OperationPlan {
	return &OperationPlan{
		Operations: []SerializableOperation{},
		Metadata: PlanMetadata{
			Version:     "1.0",
			Description: description,
		},
	}
}

// AddOperation adds a serializable operation to the plan
func (p *OperationPlan) AddOperation(op SerializableOperation) {
	p.Operations = append(p.Operations, op)
}

// ToQueue converts the plan to an executable queue
func (p *OperationPlan) ToQueue() Queue {
	queue := NewMemQueue()
	for _, op := range p.Operations {
		queue.Add(op)
	}
	return queue
}

// SerializableCreateFileOperation wraps CreateFileOperation with serialization
type SerializableCreateFileOperation struct {
	id           OperationID
	path         string
	content      []byte
	mode         fs.FileMode
	dependencies []OperationID
}

// NewSerializableCreateFile creates a new serializable create file operation
func NewSerializableCreateFile(path string, content []byte, mode fs.FileMode) *SerializableCreateFileOperation {
	return &SerializableCreateFileOperation{
		id:           OperationID(fmt.Sprintf("create_file:%s", path)),
		path:         path,
		content:      content,
		mode:         mode,
		dependencies: []OperationID{},
	}
}

// ID implements Operation
func (op *SerializableCreateFileOperation) ID() OperationID {
	return op.id
}

// Execute implements Operation
func (op *SerializableCreateFileOperation) Execute(ctx context.Context, fsys FileSystem) error {
	return fsys.WriteFile(op.path, op.content, op.mode)
}

// Validate implements Operation
func (op *SerializableCreateFileOperation) Validate(ctx context.Context, fsys FileSystem) error {
	if op.path == "" {
		return fmt.Errorf("SerializableCreateFileOperation: path cannot be empty")
	}
	if op.mode > 0777 {
		return fmt.Errorf("SerializableCreateFileOperation: invalid file mode: %o", op.mode)
	}
	return nil
}

// Dependencies implements Operation
func (op *SerializableCreateFileOperation) Dependencies() []OperationID {
	return op.dependencies
}

// Conflicts implements Operation
func (op *SerializableCreateFileOperation) Conflicts() []OperationID {
	return nil
}

// Rollback implements Operation
func (op *SerializableCreateFileOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	return fsys.Remove(op.path)
}

// Describe implements Operation
func (op *SerializableCreateFileOperation) Describe() OperationDesc {
	return OperationDesc{
		Type: "create_file",
		Path: op.path,
		Details: map[string]interface{}{
			"mode": op.mode.String(),
			"size": len(op.content),
		},
	}
}

// MarshalJSON implements SerializableOperation
func (op *SerializableCreateFileOperation) MarshalJSON() ([]byte, error) {
	data := SerializableOperationData{
		Type: "create_file",
		ID:   string(op.id),
		Dependencies: func() []string {
			deps := make([]string, len(op.dependencies))
			for i, dep := range op.dependencies {
				deps[i] = string(dep)
			}
			return deps
		}(),
		Parameters: map[string]interface{}{
			"path":    op.path,
			"content": string(op.content), // Note: this might not be suitable for binary content
			"mode":    fmt.Sprintf("%o", op.mode),
		},
	}
	return json.Marshal(data)
}

// UnmarshalJSON implements SerializableOperation
func (op *SerializableCreateFileOperation) UnmarshalJSON(data []byte) error {
	var opData SerializableOperationData
	if err := json.Unmarshal(data, &opData); err != nil {
		return err
	}

	if opData.Type != "create_file" {
		return fmt.Errorf("invalid operation type: expected 'create_file', got '%s'", opData.Type)
	}

	op.id = OperationID(opData.ID)

	// Extract parameters
	if path, ok := opData.Parameters["path"].(string); ok {
		op.path = path
	} else {
		return fmt.Errorf("missing or invalid 'path' parameter")
	}

	if content, ok := opData.Parameters["content"].(string); ok {
		op.content = []byte(content)
	} else {
		return fmt.Errorf("missing or invalid 'content' parameter")
	}

	if modeStr, ok := opData.Parameters["mode"].(string); ok {
		var mode uint64
		if _, err := fmt.Sscanf(modeStr, "%o", &mode); err != nil {
			return fmt.Errorf("invalid mode format: %s", modeStr)
		}
		op.mode = fs.FileMode(mode)
	} else {
		return fmt.Errorf("missing or invalid 'mode' parameter")
	}

	// Convert dependencies
	op.dependencies = make([]OperationID, len(opData.Dependencies))
	for i, dep := range opData.Dependencies {
		op.dependencies[i] = OperationID(dep)
	}

	return nil
}

// WithID sets a custom ID for the operation
func (op *SerializableCreateFileOperation) WithID(id OperationID) *SerializableCreateFileOperation {
	op.id = id
	return op
}

// WithDependency adds a dependency to the operation
func (op *SerializableCreateFileOperation) WithDependency(dep OperationID) *SerializableCreateFileOperation {
	op.dependencies = append(op.dependencies, dep)
	return op
}
