package local

import (
	"fmt"

	api "task-processor/internal/listingadmin"
)

type localInventoryRecordAPI struct {
	provider *LocalDataProvider
}

func (a localInventoryRecordAPI) CreateInventoryRecord(req *api.InventoryRecordCreateReqDTO) (int64, error) {
	if a.provider == nil {
		return 0, fmt.Errorf("inventory record local provider is not configured")
	}
	return a.provider.CreateInventoryRecord(req)
}

func (a localInventoryRecordAPI) GetLatestInventoryRecord(platform, productID, region string) (*api.InventoryRecordRespDTO, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("inventory record local provider is not configured")
	}
	record, _, err := a.provider.GetLatestInventoryRecord(platform, productID, region)
	return record, err
}
