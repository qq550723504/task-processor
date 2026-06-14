package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func BuildSubmitChecklist[R any, H any](readiness *SubmitReadiness[R, H]) *SubmitChecklist[R, H] {
	return sheinmarketplace.BuildSubmitChecklist(readiness, sheinmarketplace.SubmitChecklistGroupForKey)
}
