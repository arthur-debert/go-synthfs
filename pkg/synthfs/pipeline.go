package synthfs

import (
	"context"
	"fmt"

	"github.com/gammazero/toposort"
)

// Pipeline defines an interface for managing a sequence of operations.
type Pipeline interface {
	// Add appends one or more operations to the pipeline.
	// It may return an error, for example, if an operation with a duplicate ID
	// is added.
	Add(ops ...Operation) error

	// Operations returns all operations currently in the pipeline.
	// After Resolve() is called, this returns operations in dependency-resolved order.
	Operations() []Operation

	// Resolve performs dependency resolution using topological sorting.
	// This must be called before execution to ensure operations are in correct order.
	// Returns error if circular dependencies are detected.
	Resolve() error

	// Validate checks if all operations in the pipeline are valid.
	// This includes validating individual operations and checking for dependency conflicts.
	Validate(ctx context.Context, fs FileSystem) error
}

// memPipeline is an in-memory implementation of the Pipeline interface.
type memPipeline struct {
	ops      []Operation
	idIndex  map[OperationID]int // Maps operation ID to index in ops slice
	resolved bool                // Whether dependency resolution has been performed
}

// NewMemPipeline creates a new in-memory operation pipeline.
func NewMemPipeline() Pipeline {
	return &memPipeline{
		ops:      make([]Operation, 0),
		idIndex:  make(map[OperationID]int),
		resolved: false,
	}
}

// Add appends operations to the pipeline.
func (mp *memPipeline) Add(ops ...Operation) error {
	Logger().Trace().
		Interface("pipeline_add_full_context", map[string]interface{}{
			"existing_pipeline_state": func() []map[string]interface{} {
				var existing []map[string]interface{}
				for i, op := range mp.ops {
					existing = append(existing, map[string]interface{}{
						"index":        i,
						"id":           string(op.ID()),
						"type":         op.Describe().Type,
						"path":         op.Describe().Path,
						"details":      op.Describe().Details,
						"dependencies": op.Dependencies(),
						"conflicts":    op.Conflicts(),
					})
				}
				return existing
			}(),
			"new_operations": func() []map[string]interface{} {
				var newOps []map[string]interface{}
				for _, op := range ops {
					if op == nil {
						newOps = append(newOps, map[string]interface{}{
							"id":           "<nil>",
							"type":         "<nil>",
							"path":         "<nil>",
							"details":      nil,
							"dependencies": nil,
							"conflicts":    nil,
						})
					} else {
						newOps = append(newOps, map[string]interface{}{
							"id":           string(op.ID()),
							"type":         op.Describe().Type,
							"path":         op.Describe().Path,
							"details":      op.Describe().Details,
							"dependencies": op.Dependencies(),
							"conflicts":    op.Conflicts(),
						})
					}
				}
				return newOps
			}(),
			"pipeline_resolved": mp.resolved,
			"pipeline_size":     len(mp.ops),
		}).
		Msg("pipeline add operation - complete state dump")

	Logger().Info().
		Int("existing_operations", len(mp.ops)).
		Int("new_operations", len(ops)).
		Msg("adding operations to queue")

	for _, op := range ops {
		if op == nil {
			return fmt.Errorf("cannot add a nil operation to the pipeline")
		}

		// Check for duplicate IDs
		if _, exists := mp.idIndex[op.ID()]; exists {
			return fmt.Errorf("operation with ID '%s' already exists in the pipeline", op.ID())
		}

		Logger().Info().
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Str("path", op.Describe().Path).
			Int("dependencies", len(op.Dependencies())).
			Msg("operation added to queue")

		// Add operation to queue
		index := len(mp.ops)
		mp.ops = append(mp.ops, op)
		mp.idIndex[op.ID()] = index

		// Mark as unresolved since we added new operations
		mp.resolved = false
	}

	Logger().Info().
		Int("total_operations", len(mp.ops)).
		Msg("operations added to queue successfully")

	return nil
}

// Operations returns all operations currently in the pipeline.
func (mp *memPipeline) Operations() []Operation {
	// Return a copy to prevent external modification
	opsCopy := make([]Operation, len(mp.ops))
	copy(opsCopy, mp.ops)
	return opsCopy
}

// Resolve performs dependency resolution using topological sorting.
func (mp *memPipeline) Resolve() error {
	Logger().Info().
		Int("operations", len(mp.ops)).
		Bool("already_resolved", mp.resolved).
		Msg("starting dependency resolution")

	if len(mp.ops) == 0 {
		mp.resolved = true
		Logger().Info().Msg("no operations to resolve")
		return nil
	}

	if mp.resolved {
		Logger().Info().Msg("dependencies already resolved")
		return nil
	}

	// Validate that all dependencies exist
	Logger().Info().Msg("validating dependency references")
	if err := mp.validateDependencies(); err != nil {
		Logger().Info().
			Err(err).
			Msg("dependency validation failed")
		return fmt.Errorf("dependency validation failed: %w", err)
	}
	Logger().Info().Msg("dependency references validated successfully")

	// Build dependency graph using topological sort library
	edges := make([]toposort.Edge, 0)

	for _, op := range mp.ops {
		for _, depID := range op.Dependencies() {
			// Edge is [2]interface{} where element 0 comes before element 1
			// So dependency -> operation (dependency must come first)
			edges = append(edges, toposort.Edge{string(depID), string(op.ID())})
		}
	}

	Logger().Info().
		Int("dependency_edges", len(edges)).
		Msg("performing topological sort")

	// Perform topological sort
	sortedIDs, err := toposort.Toposort(edges)
	if err != nil {
		Logger().Trace().
			Interface("topological_sort_failure", map[string]interface{}{
				"edges": func() []map[string]interface{} {
					var edgeList []map[string]interface{}
					for _, edge := range edges {
						edgeList = append(edgeList, map[string]interface{}{
							"from": edge[0],
							"to":   edge[1],
						})
					}
					return edgeList
				}(),
				"operation_dependencies": func() []map[string]interface{} {
					var opDeps []map[string]interface{}
					for _, op := range mp.ops {
						opDeps = append(opDeps, map[string]interface{}{
							"id":           string(op.ID()),
							"dependencies": op.Dependencies(),
						})
					}
					return opDeps
				}(),
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
			}).
			Msg("topological sort failed - complete dependency graph dump")

		Logger().Info().
			Err(err).
			Msg("topological sort failed - circular dependency detected")
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	Logger().Trace().
		Interface("topological_sort_success", map[string]interface{}{
			"sorted_ids": sortedIDs,
			"edges": func() []map[string]interface{} {
				var edgeList []map[string]interface{}
				for _, edge := range edges {
					edgeList = append(edgeList, map[string]interface{}{
						"from": edge[0],
						"to":   edge[1],
					})
				}
				return edgeList
			}(),
			"original_order": func() []string {
				var ids []string
				for _, op := range mp.ops {
					ids = append(ids, string(op.ID()))
				}
				return ids
			}(),
		}).
		Msg("topological sort succeeded - complete sorting details")

	// Rebuild operations slice in topologically sorted order
	resolvedOps := make([]Operation, 0, len(mp.ops))
	newIdIndex := make(map[OperationID]int)

	// Add operations in dependency order
	for _, idInterface := range sortedIDs {
		idStr, ok := idInterface.(string)
		if !ok {
			return fmt.Errorf("unexpected type in topological sort result: %T", idInterface)
		}
		opID := OperationID(idStr)
		if oldIndex, exists := mp.idIndex[opID]; exists {
			newIndex := len(resolvedOps)
			resolvedOps = append(resolvedOps, mp.ops[oldIndex])
			newIdIndex[opID] = newIndex
		}
	}

	// Add any operations that weren't in the dependency graph (no dependencies or dependents)
	for _, op := range mp.ops {
		if _, alreadyAdded := newIdIndex[op.ID()]; !alreadyAdded {
			newIndex := len(resolvedOps)
			resolvedOps = append(resolvedOps, op)
			newIdIndex[op.ID()] = newIndex
		}
	}

	mp.ops = resolvedOps
	mp.idIndex = newIdIndex
	mp.resolved = true

	Logger().Info().
		Int("resolved_operations", len(resolvedOps)).
		Msg("dependency resolution completed successfully")

	return nil
}

// Validate checks if all operations in the pipeline are valid.
func (mp *memPipeline) Validate(ctx context.Context, fs FileSystem) error {
	Logger().Debug().
		Int("total_operations", len(mp.ops)).
		Bool("resolved", mp.resolved).
		Msg("starting comprehensive pipeline validation")

	// First validate dependencies exist
	Logger().Debug().Msg("validating operation dependencies")
	if err := mp.validateDependencies(); err != nil {
		Logger().Debug().
			Err(err).
			Msg("dependency validation failed")
		return err
	}
	Logger().Debug().Msg("dependency validation completed successfully")

	// Validate each operation individually
	Logger().Debug().Msg("validating individual operations")
	for i, op := range mp.ops {
		Logger().Debug().
			Int("operation_index", i+1).
			Int("total_operations", len(mp.ops)).
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Str("path", op.Describe().Path).
			Msg("validating individual operation")

		if err := op.Validate(ctx, fs); err != nil {
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("op_type", op.Describe().Type).
				Str("path", op.Describe().Path).
				Err(err).
				Msg("individual operation validation failed")
			return &ValidationError{
				Operation: op,
				Reason:    "operation validation failed",
				Cause:     err,
			}
		}

		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Str("path", op.Describe().Path).
			Msg("individual operation validation passed")
	}
	Logger().Debug().
		Int("validated_operations", len(mp.ops)).
		Msg("individual operation validation completed successfully")

	// Check for conflicts
	Logger().Debug().Msg("validating operation conflicts")
	if err := mp.validateConflicts(); err != nil {
		Logger().Debug().
			Err(err).
			Msg("conflict validation failed")
		return err
	}
	Logger().Debug().Msg("conflict validation completed successfully")

	Logger().Debug().
		Int("total_operations", len(mp.ops)).
		Msg("comprehensive pipeline validation completed successfully")

	return nil
}

// validateDependencies ensures all referenced dependencies exist in the pipeline.
func (mp *memPipeline) validateDependencies() error {
	Logger().Debug().
		Int("operations_to_check", len(mp.ops)).
		Msg("checking dependency references")

	dependencyCount := 0
	for _, op := range mp.ops {
		deps := op.Dependencies()
		dependencyCount += len(deps)

		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Interface("dependencies", deps).
			Int("dependency_count", len(deps)).
			Msg("checking operation dependencies")

		for _, depID := range deps {
			if _, exists := mp.idIndex[depID]; !exists {
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Str("missing_dependency", string(depID)).
					Interface("all_dependencies", deps).
					Msg("dependency reference validation failed - missing dependency")
				return &DependencyError{
					Operation:    op,
					Dependencies: op.Dependencies(),
					Missing:      []OperationID{depID},
				}
			} else {
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Str("dependency", string(depID)).
					Msg("dependency reference found")
			}
		}
	}

	Logger().Debug().
		Int("total_dependencies", dependencyCount).
		Int("operations_checked", len(mp.ops)).
		Msg("dependency reference validation completed")

	return nil
}

// validateConflicts checks for operations that conflict with each other.
func (mp *memPipeline) validateConflicts() error {
	Logger().Debug().
		Int("operations_to_check", len(mp.ops)).
		Msg("checking operation conflicts")

	conflictCount := 0
	for _, op := range mp.ops {
		conflicts := op.Conflicts()
		conflictCount += len(conflicts)

		if len(conflicts) > 0 {
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("op_type", op.Describe().Type).
				Interface("conflicts", conflicts).
				Int("conflict_count", len(conflicts)).
				Msg("checking operation conflicts")
		}

		for _, conflictID := range conflicts {
			if _, exists := mp.idIndex[conflictID]; exists {
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Str("conflicting_operation", string(conflictID)).
					Interface("all_conflicts", conflicts).
					Msg("conflict validation failed - conflicting operation found in queue")
				return &ConflictError{
					Operation: op,
					Conflicts: []OperationID{conflictID},
				}
			} else {
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Str("potential_conflict", string(conflictID)).
					Msg("potential conflict not in queue - no actual conflict")
			}
		}
	}

	Logger().Debug().
		Int("total_potential_conflicts", conflictCount).
		Int("operations_checked", len(mp.ops)).
		Msg("conflict validation completed - no conflicts found")

	return nil
}
