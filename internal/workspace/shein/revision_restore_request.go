package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type RestoreRequestSeed = sheinmarketplace.RestoreRequestSeed

func BuildRestoreRequestSeed(draft *EditorRevisionSkeleton) *RestoreRequestSeed {
	return sheinmarketplace.BuildRestoreRequestSeed(draft)
}
