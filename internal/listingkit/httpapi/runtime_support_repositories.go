package httpapi

import (
	"task-processor/internal/listingkit"
)

func withApprovedAssetReader(repositories BuildServiceRepositories, approvedAssets listingkit.ApprovedAssetInventoryReader) BuildServiceRepositories {
	if approvedAssets == nil {
		return repositories
	}
	repositories.Core.ApprovedAsset = approvedAssets
	return repositories
}
