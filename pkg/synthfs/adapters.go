package synthfs

import (
	"context"
	
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// OperationAdapter adapts between the main package Operation interface and the operations package interface.
// This allows gradual migration without breaking existing code.
type OperationAdapter struct {
	*operations.BaseOperation
	mainOp Operation // Reference to the main package operation for methods we don't override
}

// NewOperationAdapter creates a new adapter that wraps a main package operation.
func NewOperationAdapter(op Operation) *OperationAdapter {
	// Create a base operation with the same metadata
	baseOp := operations.NewBaseOperation(op.ID(), op.Describe().Type, op.Describe().Path)
	
	// Copy dependencies
	for _, dep := range op.Dependencies() {
		baseOp.AddDependency(dep)
	}
	
	// Copy description details
	for k, v := range op.Describe().Details {
		baseOp.SetDescriptionDetail(k, v)
	}
	
	return &OperationAdapter{
		BaseOperation: baseOp,
		mainOp:        op,
	}
}

// GetItem returns the FsItem from the main operation
func (a *OperationAdapter) GetItem() FsItem {
	return a.mainOp.GetItem()
}

// GetChecksum returns the ChecksumRecord from the main operation
func (a *OperationAdapter) GetChecksum(path string) *ChecksumRecord {
	return a.mainOp.GetChecksum(path)
}

// GetAllChecksums returns checksums from the main operation
func (a *OperationAdapter) GetAllChecksums() map[string]*ChecksumRecord {
	return a.mainOp.GetAllChecksums()
}

// Execute delegates to the main operation
func (a *OperationAdapter) Execute(ctx context.Context, fsys FileSystem) error {
	return a.mainOp.Execute(ctx, fsys)
}

// Validate delegates to the main operation
func (a *OperationAdapter) Validate(ctx context.Context, fsys FileSystem) error {
	return a.mainOp.Validate(ctx, fsys)
}

// Rollback delegates to the main operation
func (a *OperationAdapter) Rollback(ctx context.Context, fsys FileSystem) error {
	return a.mainOp.Rollback(ctx, fsys)
}

// ReverseOps delegates to the main operation
func (a *OperationAdapter) ReverseOps(ctx context.Context, fsys FileSystem, budget *core.BackupBudget) ([]Operation, *core.BackupData, error) {
	return a.mainOp.ReverseOps(ctx, fsys, budget)
}

// ConvertToOperationsPackage converts a main package operation to use the operations package interface
func ConvertToOperationsPackage(op Operation) operations.Operation {
	// For now, we'll create an adapter
	// In the future, we'll migrate operations to implement the new interface directly
	return &operationsAdapter{op: op}
}

// operationsAdapter adapts main package operations to the operations package interface
type operationsAdapter struct {
	op Operation
}

func (a *operationsAdapter) ID() core.OperationID {
	return a.op.ID()
}

func (a *operationsAdapter) Describe() core.OperationDesc {
	return a.op.Describe()
}

func (a *operationsAdapter) Dependencies() []core.OperationID {
	return a.op.Dependencies()
}

func (a *operationsAdapter) Conflicts() []core.OperationID {
	return a.op.Conflicts()
}

func (a *operationsAdapter) AddDependency(depID core.OperationID) {
	a.op.AddDependency(depID)
}

func (a *operationsAdapter) GetItem() interface{} {
	return a.op.GetItem()
}

func (a *operationsAdapter) SetItem(item interface{}) {
	if fsItem, ok := item.(FsItem); ok {
		if simpleOp, ok := a.op.(*SimpleOperation); ok {
			simpleOp.SetItem(fsItem)
		}
	}
}

func (a *operationsAdapter) GetPaths() (src, dst string) {
	if simpleOp, ok := a.op.(*SimpleOperation); ok {
		return simpleOp.GetSrcPath(), simpleOp.GetDstPath()
	}
	return "", ""
}

func (a *operationsAdapter) SetPaths(src, dst string) {
	a.op.SetPaths(src, dst)
}

func (a *operationsAdapter) GetChecksum(path string) interface{} {
	return a.op.GetChecksum(path)
}

func (a *operationsAdapter) GetAllChecksums() map[string]interface{} {
	checksums := a.op.GetAllChecksums()
	result := make(map[string]interface{})
	for k, v := range checksums {
		result[k] = v
	}
	return result
}

func (a *operationsAdapter) SetChecksum(path string, checksum interface{}) {
	if cr, ok := checksum.(*ChecksumRecord); ok {
		if simpleOp, ok := a.op.(*SimpleOperation); ok {
			simpleOp.SetChecksum(path, cr)
		}
	}
}

func (a *operationsAdapter) SetDescriptionDetail(key string, value interface{}) {
	a.op.SetDescriptionDetail(key, value)
}

func (a *operationsAdapter) Execute(ctx context.Context, fsys interface{}) error {
	if fs, ok := fsys.(FileSystem); ok {
		return a.op.Execute(ctx, fs)
	}
	return a.op.Execute(ctx, fsys.(FileSystem))
}

func (a *operationsAdapter) Validate(ctx context.Context, fsys interface{}) error {
	if fs, ok := fsys.(FileSystem); ok {
		return a.op.Validate(ctx, fs)
	}
	return a.op.Validate(ctx, fsys.(FileSystem))
}

func (a *operationsAdapter) Rollback(ctx context.Context, fsys interface{}) error {
	if fs, ok := fsys.(FileSystem); ok {
		return a.op.Rollback(ctx, fs)
	}
	return a.op.Rollback(ctx, fsys.(FileSystem))
}

func (a *operationsAdapter) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return a.op.ExecuteV2(ctx, execCtx, fsys)
}

func (a *operationsAdapter) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return a.op.ValidateV2(ctx, execCtx, fsys)
}

func (a *operationsAdapter) ReverseOps(ctx context.Context, fsys interface{}, budget interface{}) ([]interface{}, interface{}, error) {
	if fs, ok := fsys.(FileSystem); ok {
		if bb, ok := budget.(*core.BackupBudget); ok {
			ops, data, err := a.op.ReverseOps(ctx, fs, bb)
			if err != nil {
				return nil, nil, err
			}
			// Convert ops to interface{}
			result := make([]interface{}, len(ops))
			for i, op := range ops {
				result[i] = op
			}
			return result, data, nil
		}
	}
	return nil, nil, nil
}