// WithSimpleBatch sets a hint for using simple batch behavior (deprecated).
// This is a no-op for SimpleBatch since it's already the simple implementation.
func (b *SimpleBatchImpl) WithSimpleBatch(enabled bool) Batch {
	// This is a no-op for SimpleBatch since it already is the simple implementation
	return b
}

// RunWithPrerequisites runs all operations with prerequisite resolution enabled.
// For SimpleBatch, this is the same as Run() since prerequisites are always enabled.
func (b *SimpleBatchImpl) RunWithPrerequisites() (interface{}, error) {
	return b.Run() // SimpleBatch always uses prerequisite resolution
}

// RunWithPrerequisitesAndBudget runs all operations with prerequisite resolution and backup enabled.
func (b *SimpleBatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	return b.RunRestorableWithBudget(maxBackupMB)
}

// Helper methods