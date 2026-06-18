package sheinmanaged

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheincategory "task-processor/internal/shein/api/category"
)

func buildCategoryAPI(factory *apiFactory, storeID int64) (sheinpub.CategoryAPI, string) {
	baseAPIClient, note := factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheincategory.NewClient(baseAPIClient), ""
}
