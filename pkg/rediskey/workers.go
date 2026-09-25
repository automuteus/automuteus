package rediskey

// These keys are shared across bot processes, so maintenance has one fleet-wide budget.
const WorkerCleanupLease = "automuteus:workers:cleanup:lease"
const WorkerCleanupSchedule = "automuteus:workers:cleanup:schedule"
const WorkerCleanupPending = "automuteus:workers:cleanup:pending"
const WorkerCleanupScanBudget = "automuteus:workers:cleanup:scan-budget"
const WorkerCleanupLeaveBudget = "automuteus:workers:cleanup:leave-budget"
const WorkerCleanupInventoryBudget = "automuteus:workers:cleanup:inventory-budget"

func WorkerCleanupPause(tokenHash string) string {
	return "automuteus:workers:cleanup:pause:" + tokenHash
}
