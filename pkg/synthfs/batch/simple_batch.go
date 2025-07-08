// RunWithPrerequisitesAndBudget runs all operations with prerequisite resolution and backup enabled.
func (sb *SimpleBatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	opts := map[string]interface{}{
		"resolve_prerequisites": true,
		"restorable":            true,
		"max_backup_size_mb":    maxBackupMB,
	}
	return sb.RunWithOptions(opts)
}

// RunWithSimpleBatch runs all operations with SimpleBatch behavior.
// Note: This is the default behavior for SimpleBatchImpl, so it's equivalent to Run().
func (sb *SimpleBatchImpl) RunWithSimpleBatch() (interface{}, error) {
	return sb.Run()
}

// RunWithSimpleBatchAndBudget runs all operations with SimpleBatch behavior and backup enabled.
func (sb *SimpleBatchImpl) RunWithSimpleBatchAndBudget(maxBackupMB int) (interface{}, error) {
	return sb.RunRestorableWithBudget(maxBackupMB)
}

// RunWithLegacyBatch runs all operations with legacy batch behavior.
// Note: SimpleBatch doesn't support legacy behavior - this delegates to SimpleBatch.
func (sb *SimpleBatchImpl) RunWithLegacyBatch() (interface{}, error) {
	return sb.Run()
}

// RunWithLegacyBatchAndBudget runs all operations with legacy batch behavior and backup enabled.
// Note: SimpleBatch doesn't support legacy behavior - this delegates to SimpleBatch.
func (sb *SimpleBatchImpl) RunWithLegacyBatchAndBudget(maxBackupMB int) (interface{}, error) {
	return sb.RunRestorableWithBudget(maxBackupMB)
}

// Helper methods