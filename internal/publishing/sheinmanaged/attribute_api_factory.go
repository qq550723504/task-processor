package sheinmanaged

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func buildAttributeAPI(factory *apiFactory, storeID int64) (sheinpub.AttributeAPI, string) {
	baseAPIClient, note := factory.BuildBaseClient(storeID)
	if baseAPIClient == nil {
		return nil, note
	}
	return sheinattribute.NewClient(baseAPIClient), ""
}
