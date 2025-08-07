package core

import "fmt"

// ValidationError represents an error during operation validation.
// This is moved from the main package to break circular dependencies.
type ValidationError struct {
	OperationID   OperationID
	OperationDesc OperationDesc
	Reason        string
	Cause         error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error for operation %s (%s): %s: %v",
			e.OperationID, e.OperationDesc.Path, e.Reason, e.Cause)
	}
	return fmt.Sprintf("validation error for operation %s (%s): %s",
		e.OperationID, e.OperationDesc.Path, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// RollbackError represents an error that occurs during a rollback,
// wrapping the original execution error.
// This is the unified version with richer error tracking.
type RollbackError struct {
	OriginalErr  error
	RollbackErrs map[OperationID]error // Map from operation ID to rollback error
}

func (e *RollbackError) Error() string {
	msg := fmt.Sprintf("Operation failed: %v", e.OriginalErr)

	if len(e.RollbackErrs) > 0 {
		msg += "\n\nRollback also failed:"
		for id, err := range e.RollbackErrs {
			msg += fmt.Sprintf("\n  - %s: %v", id, err)
		}
	}

	return msg
}

func (e *RollbackError) Unwrap() error {
	return e.OriginalErr
}
