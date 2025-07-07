package batch

import "time"

// ResultImpl represents the outcome of executing a batch of operations.
type ResultImpl struct {
	success    bool
	operations []interface{}
	restoreOps []interface{}
	duration   time.Duration
	err        error
}

// NewResult creates a new batch result.
func NewResult(success bool, operations []interface{}, restoreOps []interface{}, duration time.Duration, err error) Result {
	return &ResultImpl{
		success:    success,
		operations: operations,
		restoreOps: restoreOps,
		duration:   duration,
		err:        err,
	}
}

// IsSuccess returns whether the batch execution was successful.
func (r *ResultImpl) IsSuccess() bool {
	return r.success
}

// GetOperations returns the operations that were executed.
func (r *ResultImpl) GetOperations() []interface{} {
	return r.operations
}

// GetRestoreOps returns the restore operations if any.
func (r *ResultImpl) GetRestoreOps() []interface{} {
	return r.restoreOps
}

// GetDuration returns the execution duration.
func (r *ResultImpl) GetDuration() interface{} {
	return r.duration
}

// GetError returns any error that occurred during execution.
func (r *ResultImpl) GetError() error {
	return r.err
}