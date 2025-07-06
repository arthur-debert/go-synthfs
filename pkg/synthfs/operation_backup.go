package synthfs

import (
	"fmt"
)

// ConsumeBackup reduces the remaining budget by the specified amount
func (bb *BackupBudget) ConsumeBackup(sizeMB float64) error {
	if sizeMB > bb.RemainingMB {
		return fmt.Errorf("backup size %.2fMB exceeds remaining budget %.2fMB", sizeMB, bb.RemainingMB)
	}
	bb.RemainingMB -= sizeMB
	bb.UsedMB += sizeMB
	return nil
}

// RestoreBackup increases the remaining budget by the specified amount
func (bb *BackupBudget) RestoreBackup(sizeMB float64) {
	bb.RemainingMB += sizeMB
	bb.UsedMB -= sizeMB
	if bb.UsedMB < 0 {
		bb.UsedMB = 0
	}
	if bb.RemainingMB > bb.TotalMB {
		bb.RemainingMB = bb.TotalMB
	}
}